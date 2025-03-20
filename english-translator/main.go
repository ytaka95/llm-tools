package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	// コマンドライン引数のチェック
	if len(os.Args) < 2 {
		fmt.Println("引数に翻訳したい日本語の文章を指定してください")
		os.Exit(1)
	}

	// コマンドライン引数からプロンプトを取得
	targetText := os.Args[1]

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	var maxTokens int32 = 100
	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: "Please translate the following Japanese text into English.\n<requirements><req>The translation should be somewhat formal, suitable for a chat message to a colleague within a company.</req><req>The translation should be natural English, not a literal translation.</req><req>The output should only be the translated English sentence.</req></requirements>"},
		},
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens:   &maxTokens,
		SystemInstruction: systemInstruction,
	}

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-lite", genai.Text("<text_to_translate>" + targetText + "</text_to_translate>"), config)
	if err != nil {
		log.Fatal(err)
	}

	printResponse(result)
}

func printResponse(resp *genai.GenerateContentResponse) {
	fmt.Println("✓ Translated text:")
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			fmt.Print(part.Text)
		}
	}
	fmt.Println("✓ CreateTime: ", resp.CreateTime)
	fmt.Println("✓ ModelVersion: ", resp.ModelVersion)
	fmt.Println("✓ UsageMetadata:")
	fmt.Println("  - CachedContentTokenCount: ", resp.UsageMetadata.CachedContentTokenCount)
	fmt.Println("  - CandidatesTokenCount: ", resp.UsageMetadata.CandidatesTokenCount)
	fmt.Println("  - PromptTokenCount: ", resp.UsageMetadata.PromptTokenCount)
	fmt.Println("  - TotalTokenCount: ", resp.UsageMetadata.TotalTokenCount)
}
