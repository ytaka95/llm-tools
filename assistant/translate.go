package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
	"google.golang.org/genai"
)

// システム指示、モデル、入力テキストなどのLLMリクエスト設定
type LlmRequestConfig struct {
	SystemInstruction string
	Model             string
	MaxTokens         int32
	InputText         string
	IncludeThoughts   bool
	ThinkingBudget    *int32
	ThinkingLevel     genai.ThinkingLevel
}

// LLMリクエストに関するメタデータ
type LLMMetadata struct {
	APICallTime          time.Duration
	ModelVersion         string
	PromptTokenCount     int32
	CandidatesTokenCount int32
	ThoughtsTokenCount   int32
	TotalTokenCount      int32
}

// generateContentをサポートする利用可能なモデルを標準エラー出力にリストする
func listAvailableModels(ctx context.Context, client *genai.Client) {
	pageSize := int32(20)
	var listModelsConfig = genai.ListModelsConfig{
		PageSize: pageSize,
	}
	iter, err := client.Models.List(ctx, &listModelsConfig)
	if err != nil {
		log.Printf("Error listing models: %v", err)
		return
	}

	for {
		models := iter.Items
		for _, m := range models {
			supportsGenerateContent := false
			if slices.Contains(m.SupportedActions, "generateContent") {
				supportsGenerateContent = true
			}

			if supportsGenerateContent {
				fmt.Fprintln(os.Stderr, "- ", m.Name, "\n    ", m.Description)
			}
		}
		iter, err = iter.Next(ctx)
		if err == genai.ErrPageDone {
			break
		}
		if err != nil {
			log.Printf("Error going to next page: %v", err)
			break
		}
	}
}

// 設定に基づいてクライアントをGemini APIまたはVertex AIクライアントとして初期化する
func initClient(ctx context.Context, settings *Settings) (*genai.Client, string, error) {
	switch settings.APIMethod {
	case "apiKey":
		// APIキーを使う場合
		apiKey := os.Getenv(settings.APIKeyConfig.APIKeyEnvVarName)
		if apiKey == "" {
			return nil, "", fmt.Errorf("環境変数 '%s' にAPIキーが設定されていません", settings.APIKeyConfig.APIKeyEnvVarName)
		}

		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, "", fmt.Errorf("Gemini APIクライアントの初期化に失敗しました: %w", err)
		}
		return client, "Gemini API", nil

	case "vertexAI":
		// Vertex AIを使う場合
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			Project:  settings.VertexAIConfig.Project,
			Location: settings.VertexAIConfig.Location,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			return nil, "", fmt.Errorf("Vertex AIクライアントの初期化に失敗しました: %w", err)
		}
		return client, "Vertex AI", nil

	default:
		return nil, "", fmt.Errorf("無効なAPIメソッド: %s", settings.APIMethod)
	}
}

func parseThinkingLevel(level string) (genai.ThinkingLevel, error) {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "minimal":
		return genai.ThinkingLevelMinimal, nil
	case "low":
		return genai.ThinkingLevelLow, nil
	case "medium":
		return genai.ThinkingLevelMedium, nil
	case "high":
		return genai.ThinkingLevelHigh, nil
	default:
		return "", fmt.Errorf("無効な -think-level が指定されました: %s (指定可能: minimal|low|medium|high)", level)
	}
}

func isGemini3ProModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	name = strings.TrimPrefix(name, "models/")
	return strings.HasPrefix(name, "gemini-3-pro")
}

// TaskDefinitionに基づいてLlmRequestConfigとgenai.GenerateContentConfigを作成する
func createLLMConfigs(task TaskDefinition, modelName string, inputText string, enableThinking bool, requestedThinkingLevel string) (LlmRequestConfig, *genai.GenerateContentConfig, error) {
	var includeThoughts = false
	var thinkingBudgetValue int32 = 0
	var thinkingBudget *int32
	var thinkingLevel genai.ThinkingLevel

	isGemini3 := isGemini3Model(modelName)
	isGemini3Pro := isGemini3ProModel(modelName)

	if isGemini3 {
		if strings.TrimSpace(requestedThinkingLevel) != "" {
			parsedLevel, err := parseThinkingLevel(requestedThinkingLevel)
			if err != nil {
				return LlmRequestConfig{}, nil, err
			}
			thinkingLevel = parsedLevel
		} else if enableThinking {
			thinkingLevel = genai.ThinkingLevelHigh
		} else {
			thinkingLevel = genai.ThinkingLevelLow
		}

		if isGemini3Pro && thinkingLevel != genai.ThinkingLevelLow && thinkingLevel != genai.ThinkingLevelHigh {
			return LlmRequestConfig{}, nil, fmt.Errorf("モデル '%s' では -think-level は low または high のみ指定可能です", modelName)
		}
	} else if enableThinking {
		thinkingBudgetValue = 1024
		thinkingBudget = &thinkingBudgetValue
	} else {
		thinkingBudget = &thinkingBudgetValue
	}

	includeThoughts = enableThinking

	maxTokens := int32(len(inputText))*task.MaxTokensMultiplier + task.MaxTokensBase
	if thinkingBudget != nil {
		maxTokens += *thinkingBudget
	}

	llmRequestConfig := LlmRequestConfig{
		SystemInstruction: task.SystemInstruction,
		Model:             modelName,
		MaxTokens:         maxTokens,
		InputText:         task.InputPrefix + html.EscapeString(inputText) + task.InputSuffix,
		IncludeThoughts:   includeThoughts,
		ThinkingBudget:    thinkingBudget,
		ThinkingLevel:     thinkingLevel,
	}

	var config *genai.GenerateContentConfig
	if isGemini3 {
		config = &genai.GenerateContentConfig{
			MaxOutputTokens: llmRequestConfig.MaxTokens,
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: llmRequestConfig.SystemInstruction},
				},
			},
			ThinkingConfig: &genai.ThinkingConfig{
				IncludeThoughts: llmRequestConfig.IncludeThoughts,
				ThinkingLevel:   llmRequestConfig.ThinkingLevel,
			},
		}
	} else {
		config = &genai.GenerateContentConfig{
			MaxOutputTokens: llmRequestConfig.MaxTokens,
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: llmRequestConfig.SystemInstruction},
				},
			},
			ThinkingConfig: &genai.ThinkingConfig{
				IncludeThoughts: llmRequestConfig.IncludeThoughts,
				ThinkingBudget:  llmRequestConfig.ThinkingBudget,
			},
		}
	}
	return llmRequestConfig, config, nil
}

func isGemini3Model(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	name = strings.TrimPrefix(name, "models/")
	return strings.HasPrefix(name, "gemini-3")
}

// Gemini APIにリクエストを送信し、ストリームされたコンテンツをoutputChanに送信する
// メタデータを収集し、エラーが発生した場合はそれを返す
func streamContent(ctx context.Context, client *genai.Client, llmReqConfig LlmRequestConfig, genaiConfig *genai.GenerateContentConfig, outputChan chan<- string) (LLMMetadata, error) {
	start := time.Now()
	stream := client.Models.GenerateContentStream(ctx, llmReqConfig.Model, genai.Text(llmReqConfig.InputText), genaiConfig)

	var metadata LLMMetadata

	// ストリームから結果を読み込み、出力チャネルに送信
	for result, err := range stream {
		if err != nil {
			// エラーメッセージが404を含む場合、モデル一覧を表示する
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
				fmt.Fprintln(os.Stderr, err.Error())
				listAvailableModels(ctx, client)
				return metadata, fmt.Errorf("指定されたモデル '%s' が見つからないか、generateContentをサポートしていません: %w", llmReqConfig.Model, err)
			}
			// その他のエラーの場合はそのまま返す
			return metadata, fmt.Errorf("API呼び出し中にエラーが発生しました: %w", err)
		}

		// // デバッグ: レスポンス構造を出力
		// if result != nil {
		// 	resultJSON, _ := json.MarshalIndent(result, "", "  ")
		// 	fmt.Fprintf(os.Stderr, "\n==== DEBUG: API Response Structure ====\n")
		// 	fmt.Fprintf(os.Stderr, "%s\n", string(resultJSON))
		// 	fmt.Fprintf(os.Stderr, "=====================================\n\n")
		// }

		// 結果を出力
		if result != nil && result.Candidates != nil {
			for _, cand := range result.Candidates {
				if cand != nil && cand.Content != nil && cand.Content.Parts != nil {
					for _, part := range cand.Content.Parts {
						if part != nil && part.Text != "" {
							var text string
							if part.Thought == true {
								text = color.BlueString(html.UnescapeString(part.Text))
							} else {
								text = html.UnescapeString(part.Text)
							}
							outputChan <- text
						}
					}
				}
			}
		}

		// メタデータを更新
		if result != nil {
			metadata.ModelVersion = result.ModelVersion
			if result.UsageMetadata != nil {
				metadata.TotalTokenCount = result.UsageMetadata.TotalTokenCount
				metadata.PromptTokenCount = result.UsageMetadata.PromptTokenCount
				metadata.CandidatesTokenCount = result.UsageMetadata.CandidatesTokenCount
				metadata.ThoughtsTokenCount = result.UsageMetadata.ThoughtsTokenCount
			}
		}
	}
	metadata.APICallTime = time.Since(start)

	return metadata, nil
}

// メタデータを出力
func printMetadata(metadata LLMMetadata, apiMethod string, taskName string) {
	fmt.Fprintln(os.Stderr, "==== Metadata ====")
	fmt.Fprintln(os.Stderr, "✓ Task:                  ", taskName)
	fmt.Fprintln(os.Stderr, "✓ API method:            ", apiMethod)
	fmt.Fprintln(os.Stderr, "✓ API call time:         ", metadata.APICallTime)
	fmt.Fprintln(os.Stderr, "✓ Model version:         ", metadata.ModelVersion)
	fmt.Fprintln(os.Stderr, "✓ Prompt token count:    ", metadata.PromptTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Candidate token count: ", metadata.CandidatesTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Thoughts token count:  ", metadata.ThoughtsTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Total token count:     ", metadata.TotalTokenCount)
	fmt.Fprintln(os.Stderr, "==================")
}
