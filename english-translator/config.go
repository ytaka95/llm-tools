package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Vertex AI接続の設定
type VertexAIConfig struct {
	Project  string `json:"project"`
	Location string `json:"location"`
}

// APIキー接続の設定
type APIKeyConfig struct {
	APIKeyEnvVarName string `json:"apiKeyEnvVarName"`
}

// アプリケーションの全体設定
type Settings struct {
	APIMethod      string         `json:"apiMethod"` // "apiKey" または "vertexAI"
	VertexAIConfig VertexAIConfig `json:"vertexAiConfig"`
	APIKeyConfig   APIKeyConfig   `json:"apiKeyConfig"`
}

// 設定ファイルのパスを返す
func getSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ホームディレクトリの取得に失敗しました: %w", err)
	}
	return filepath.Join(homeDir, ".config", "llm-english-translator", "settings.json"), nil
}

// 設定ファイルディレクトリを作成する
func ensureSettingsDir() error {
	settingsPath, err := getSettingsPath()
	if err != nil {
		return err
	}
	settingsDir := filepath.Dir(settingsPath)
	return os.MkdirAll(settingsDir, 0755)
}

// 設定ファイルから設定を読み込む
func loadSettings() (*Settings, error) {
	settingsPath, err := getSettingsPath()
	if err != nil {
		return nil, err
	}

	// ファイルが存在しない場合はnilを返す
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("設定ファイルの解析に失敗しました: %w", err)
	}

	return &settings, nil
}

// 設定をファイルに保存する
func saveSettings(settings *Settings) error {
	if err := ensureSettingsDir(); err != nil {
		return err
	}

	settingsPath, err := getSettingsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("設定のシリアライズに失敗しました: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("設定ファイルの保存に失敗しました: %w", err)
	}

	return nil
}

// 対話型で設定をセットアップする
func setupInteractive() (*Settings, error) {
	fmt.Println("設定ファイルが見つかりません。対話形式で設定を行います。")
	scanner := bufio.NewScanner(os.Stdin)

	// APIメソッドの選択
	fmt.Println()
	fmt.Println("使用するAPIメソッドを選択してください:")
	fmt.Println("1. APIキーを使用 (Gemini API)")
	fmt.Println("2. Vertex AIを使用")
	fmt.Print("選択してください (1または2): ")

	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	settings := &Settings{}

	switch choice {
	case "1":
		settings.APIMethod = "apiKey"

		// APIキー環境変数名の設定
		fmt.Print("APIキーが設定されている環境変数名を入力してください (デフォルト: API_KEY_GOOGLE): ")
		scanner.Scan()
		envVarName := strings.TrimSpace(scanner.Text())
		if envVarName == "" {
			envVarName = "API_KEY_GOOGLE"
		}
		settings.APIKeyConfig.APIKeyEnvVarName = envVarName

	case "2":
		settings.APIMethod = "vertexAI"

		// プロジェクトIDの設定
		fmt.Print("Google CloudプロジェクトIDを入力してください: ")
		scanner.Scan()
		project := strings.TrimSpace(scanner.Text())
		if project == "" {
			return nil, fmt.Errorf("プロジェクトIDは必須です")
		}
		settings.VertexAIConfig.Project = project

		// リージョンの設定
		fmt.Print("Vertex AIのリージョンを入力してください (デフォルト: asia-northeast1): ")
		scanner.Scan()
		location := strings.TrimSpace(scanner.Text())
		if location == "" {
			location = "asia-northeast1"
		}
		settings.VertexAIConfig.Location = location

	default:
		return nil, fmt.Errorf("無効な選択です: %s", choice)
	}

	// 設定を保存
	if err := saveSettings(settings); err != nil {
		return nil, fmt.Errorf("設定の保存に失敗しました: %w", err)
	}

	fmt.Println()
	fmt.Printf("設定を ~/.config/llm-english-translator/settings.json に保存しました。\n")
	fmt.Printf("選択したAPIメソッド: %s\n", settings.APIMethod)
	fmt.Println()

	return settings, nil
}
