package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
}

func main() {
	// コマンドライン引数のチェック
	if len(os.Args) < 2 {
		fmt.Println("引数に翻訳したい日本語の文章を指定してください")
		os.Exit(1)
	}

	// コマンドライン引数からプロンプトを取得
	targetText := os.Args[1]

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
		Model: "gemini-2.0-flash-lite",
		MaxTokens: int32(len(targetText) * 10),
		InputText: "<text_to_translate>" + html.EscapeString(targetText) + "</text_to_translate>",
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens:   &llmRequestConfig.MaxTokens,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: llmRequestConfig.SystemInstruction},
			},
		},
	}

	start := time.Now()
	stream := client.Models.GenerateContentStream(ctx, llmRequestConfig.Model, genai.Text(llmRequestConfig.InputText), config)

	var output string
	var modelVersion string
	var totalTokenCount int32

	for result, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		for _, cand := range result.Candidates {
			for _, part := range cand.Content.Parts {
				text := html.UnescapeString(part.Text)
				fmt.Print(text)
				output += text
			}
		}
		modelVersion = result.ModelVersion
		totalTokenCount = result.UsageMetadata.TotalTokenCount
	}

	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}
	fmt.Fprintln(os.Stderr, "==== Metadata ====")
	fmt.Fprintln(os.Stderr, "✓ API call time:     ", time.Since(start))
	fmt.Fprintln(os.Stderr, "✓ Model version:     ", modelVersion)
	fmt.Fprintln(os.Stderr, "✓ Total token count: ", totalTokenCount)
	fmt.Fprintln(os.Stderr, "==================")
}
