package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"time"
	"html"
	"strings"

	"google.golang.org/genai"
)

// system instruction, model, input textを定数として保持するデータ構造
type LlmRequestConfig struct {
	SystemInstruction string
	Model             string
	MaxTokens         int32
	InputText         string
	IncludeThoughts   bool
	ThinkingBudget    int32
}

func listAvailableModels(ctx context.Context, client *genai.Client) {
	fmt.Fprintln(os.Stderr, "Error: Model not found. Available models supporting 'generateContent':")

	pageSize := int32(20)
	var listModelsConfig = genai.ListModelsConfig{
		PageSize: pageSize,
	}
	iter, err := client.Models.List(ctx, &listModelsConfig)
	// 最初に終了条件を確認
	if err == genai.ErrPageDone {
		return
	}
	if err != nil {
		log.Printf("Error creating client: %v", err)
		return // エラーが発生したら処理を中断
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

func main() {
	// コマンドライン引数の定義
	var modelName string
	flag.StringVar(&modelName, "m", "gemini-2.5-flash-preview-04-17", "モデル名を指定します")

	// flagの解析（これで-mオプションを解析する）
	flag.Parse()

	// 残りの引数（翻訳テキスト）を取得
	args := flag.Args()

	// 出力用のチャネルを作成
	outputChan := make(chan string, 100)
	done := make(chan bool)

	// 出力処理を別ゴルーチンで実行
	go func() {
		for text := range outputChan {
			charLengthPerStep := 5
			timePerChar := 50 * time.Millisecond
			var start = 0
			for start < len(text) {
				end := min(start + charLengthPerStep, len(text))
				fmt.Print(text[start:end])
				start = end
				time.Sleep(timePerChar)
			}
		}
		done <- true
	}()

	// コマンドライン引数のチェック
	if len(args) < 1 {
		fmt.Println("引数に翻訳したい日本語の文章を指定してください")
		fmt.Println("使用方法: english-translator [-m model_name] \"翻訳したい日本語\"")
		os.Exit(1)
	}

	// コマンドライン引数からプロンプトを取得
	targetText := args[0]

	if os.Getenv("API_KEY_GOOGLE") == "" {
		fmt.Println("環境変数 API_KEY_GOOGLE を設定してください")
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("API_KEY_GOOGLE"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	// LlmRequestConfigの初期化
	llmRequestConfig := LlmRequestConfig{
		SystemInstruction: "Please translate the following Japanese text into English.\n<requirements><req>The translation should be somewhat formal, suitable for a chat message to a colleague, a documentation within a company, or simple and short git commit message.</req><req>The translation should be natural English, not a literal translation.</req><req>The output should only be the translated English sentence.</req><req>Keep the original formatting (e.g., Markdown) of the text.</req><req>The original Japanese text may contain XML tags and emoji, which should be preserved in the output.</req></requirements>",
		Model: modelName,
		MaxTokens: int32(len(targetText) * 10),
		InputText: "<text_to_translate>" + html.EscapeString(targetText) + "</text_to_translate>",
		IncludeThoughts: false,
		ThinkingBudget: 0,
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens:   llmRequestConfig.MaxTokens,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: llmRequestConfig.SystemInstruction},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: llmRequestConfig.IncludeThoughts,
			ThinkingBudget: &llmRequestConfig.ThinkingBudget,
		},
	}

	start := time.Now()
	stream := client.Models.GenerateContentStream(ctx, llmRequestConfig.Model, genai.Text(llmRequestConfig.InputText), config)

	var lastText = ""
	var modelVersion string
	// var totalTokenCount int32
	var promptTokenCount int32
	var candidatesTokenCount int32
	var thoughtsTokenCount int32

	// ストリームから結果を読み込み、出力チャネルに送信
	for result, err := range stream {
		if err != nil {
			// エラーメッセージが404を含む場合、モデル一覧を表示する
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
				listAvailableModels(ctx, client)
				os.Exit(1)
			}
			// その他のエラーの場合はそのまま表示
			log.Fatal(err)
		}

		// 結果がnilでないか、候補があるかなどをチェック
		if result != nil && result.Candidates != nil {
			for _, cand := range result.Candidates {
				// 候補がnilでないか、コンテンツがあるかなどをチェック
				if cand != nil && cand.Content != nil && cand.Content.Parts != nil {
					for _, part := range cand.Content.Parts {
						// パートがnilでないか、テキストがあるかなどをチェック
						if part != nil && part.Text != "" {
							text := html.UnescapeString(part.Text)
							outputChan <- text
							lastText = text
						}
					}
				}
			}
		}

		// メタデータを更新 (nilチェックを追加)
		if result != nil {
			modelVersion = result.ModelVersion
			if result.UsageMetadata != nil {
				// totalTokenCount = result.UsageMetadata.TotalTokenCount
				promptTokenCount = result.UsageMetadata.PromptTokenCount
				candidatesTokenCount = result.UsageMetadata.CandidatesTokenCount
				thoughtsTokenCount = result.UsageMetadata.ThoughtsTokenCount
			}
		}
	}
	apiCallTime := time.Since(start)

	// チャネルをクローズして出力処理の終了を待つ
	close(outputChan)
	<-done

	if lastText != "" && !strings.HasSuffix(lastText, "\n") {
		fmt.Println()
	}
	fmt.Fprintln(os.Stderr, "==== Metadata ====")
	fmt.Fprintln(os.Stderr, "✓ API call time:     ", apiCallTime)
	fmt.Fprintln(os.Stderr, "✓ Model version:     ", modelVersion)
	// fmt.Fprintln(os.Stderr, "✓ Total token count: ", totalTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Prompt token count: ", promptTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Candidate token count: ", candidatesTokenCount)
	fmt.Fprintln(os.Stderr, "✓ Thoughts token count: ", thoughtsTokenCount)
	fmt.Fprintln(os.Stderr, "==================")
}
