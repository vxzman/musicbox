package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const GeneralConfig = "config_generic.json"

// ─── 配置同步 ────────────────────────────────────────────────
// 单一事实源：config_generic.json（用户代理配置） + manager.yaml 各模式
// preset（预定义入站 JSON 数组）。SyncAll 将两者合并生成全部 config_<mode>.json。

func (c *ManagerConfig) GeneralPath() string {
	return filepath.Join(c.ConfigDir(), GeneralConfig)
}

func (c *ManagerConfig) ModeConfigPath(mode string) (string, error) {
	m, ok := c.Modes[mode]
	if !ok {
		return "", fmt.Errorf("未知模式: %s", mode)
	}
	return filepath.Join(c.ConfigDir(), m.Config), nil
}

func ReadGeneral(c *ManagerConfig) (string, error) {
	data, err := os.ReadFile(c.GeneralPath())
	if err != nil {
		return "", fmt.Errorf("读取通用配置失败: %w", err)
	}
	return string(data), nil
}

func ReadModeConfig(c *ManagerConfig, mode string) (string, error) {
	path, err := c.ModeConfigPath(mode)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取配置 %s 失败: %w", mode, err)
	}
	return string(data), nil
}

// SaveGeneral 保存通用配置并同步全部模式配置。
func SaveGeneral(c *ManagerConfig, content string) error {
	if err := testSingBoxConfig(c, content); err != nil {
		return fmt.Errorf("sing-box 配置测试失败: %w", err)
	}
	if err := os.WriteFile(c.GeneralPath(), []byte(content), 0644); err != nil {
		return fmt.Errorf("写入通用配置失败: %w", err)
	}
	return SyncAll(c, content)
}

// SyncAll 读取当前通用配置（content 为空则读盘），生成全部模式配置。
func SyncAll(c *ManagerConfig, content string) error {
	if content == "" {
		var err error
		content, err = ReadGeneral(c)
		if err != nil {
			return err
		}
	}
	general, err := parseGeneral(content)
	if err != nil {
		return err
	}
	for name := range c.Modes {
		if c.Modes[name].Preset == "" {
			continue // 用户自管模式（server）：不生成、不覆盖
		}
		cfg, err := buildModeConfig(general, c.Modes[name].Preset)
		if err != nil {
			return fmt.Errorf("生成 %s 配置失败: %w", name, err)
		}
		if err := testSingBoxConfig(c, string(cfg)); err != nil {
			return fmt.Errorf("%s 配置测试失败: %w", name, err)
		}
		path, err := c.ModeConfigPath(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, cfg, 0644); err != nil {
			return fmt.Errorf("写入 %s 配置失败: %w", name, err)
		}
	}
	return nil
}

// ─── 合并逻辑 ────────────────────────────────────────────────

func parseGeneral(content string) (map[string]interface{}, error) {
	var general map[string]interface{}
	if err := json.Unmarshal([]byte(content), &general); err != nil {
		return nil, fmt.Errorf("通用配置 JSON 语法错误: %w", err)
	}
	if general == nil {
		return nil, fmt.Errorf("通用配置为空")
	}
	return general, nil
}

// ValidatePreset 检查预定义入站是否为非空 JSON 数组，且每项都有 tag。
func ValidatePreset(preset string) error {
	var inbounds []map[string]interface{}
	if err := json.Unmarshal([]byte(preset), &inbounds); err != nil {
		return fmt.Errorf("JSON 语法错误: %w", err)
	}
	if len(inbounds) == 0 {
		return fmt.Errorf("入站模块必须是至少包含一项的数组")
	}
	for i, in := range inbounds {
		if tag, _ := in["tag"].(string); tag == "" {
			return fmt.Errorf("入站模块第 %d 项缺少 tag", i+1)
		}
	}
	return nil
}

// buildModeConfig 将通用配置与模式 preset 合并：用户自写 inbounds 保留，
// preset 中同 tag 覆盖、缺 tag 追加。
func buildModeConfig(general map[string]interface{}, preset string) ([]byte, error) {
	var presetInbounds []map[string]interface{}
	if err := json.Unmarshal([]byte(preset), &presetInbounds); err != nil {
		return nil, fmt.Errorf("解析预定义配置失败: %w", err)
	}

	cfg := make(map[string]interface{}, len(general)+1)
	for k, v := range general {
		cfg[k] = v
	}

	// 合并 inbounds：preset 的同 tag 项覆盖通用配置里的同名项
	merged := []interface{}{}
	byTag := map[string]int{}
	if existing, ok := cfg["inbounds"].([]interface{}); ok {
		for _, in := range existing {
			if m, ok := in.(map[string]interface{}); ok {
				if tag, ok := m["tag"].(string); ok && tag != "" {
					byTag[tag] = len(merged)
				}
			}
			merged = append(merged, in)
		}
	}
	for _, in := range presetInbounds {
		tag, _ := in["tag"].(string)
		if tag != "" {
			if idx, exists := byTag[tag]; exists {
				merged[idx] = in // 覆盖
				continue
			}
			byTag[tag] = len(merged)
		}
		merged = append(merged, in)
	}
	cfg["inbounds"] = merged

	// JSON 不支持注释，仅做美化输出
	return json.MarshalIndent(cfg, "", "  ")
}

// testSingBoxConfig 用 sing-box check 校验配置；机器上没有 sing-box（如 CI）则跳过。
// 老面板的做法：root 时以 sing-box 用户身份执行（权限问题在校验期暴露）。
// 防挂起：整个校验包在 20s 超时内。
func testSingBoxConfig(c *ManagerConfig, content string) error {
	bin, err := exec.LookPath("sing-box")
	if err != nil {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "singbox-manager-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return err
	}

	// 工作目录：数据目录存在则用之（cache_file 等相对路径），否则临时目录
	workDir := c.DataDir()
	if _, err := os.Stat(workDir); err != nil {
		workDir = tmpDir
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// root 时优先以 sing-box 用户身份降权校验（权限问题在校验期暴露，
	// 老面板的做法）。能力集受限（无 CAP_SETUID/SETGID）时 runuser 会失败，
	// 降级为 root 直接校验，不让校验阻塞配置保存。
	if os.Geteuid() == 0 {
		if _, err := exec.LookPath("runuser"); err == nil {
			cmd := exec.CommandContext(ctx, "runuser", "-u", "sing-box", "--", bin, "check", "-D", workDir, "-c", tmpFile)
			out, err := cmd.CombinedOutput()
			if err == nil {
				return nil
			}
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("配置校验超时（20s）")
			}
			log.Printf("[config] runuser 降权校验失败（%s），改用 root 校验: %s",
				strings.TrimSpace(string(out)), err)
		}
	}

	cmd := exec.CommandContext(ctx, bin, "check", "-D", workDir, "-c", tmpFile)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("配置校验超时（20s）")
	}
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
