package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
		APIKey: os.Getenv("API_KEY_GOOGLE"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	var maxTokens int32 = int32(len(targetText) * 10)
	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: "Please translate the following Japanese text into English.\n<requirements><req>The translation should be somewhat formal, suitable for a chat message to a colleague  or a documentation within a company.</req><req>The translation should be natural English, not a literal translation.</req><req>The output should only be the translated English sentence.</req><req>Keep the original formatting (e.g., Markdown) of the text.</req><req>The original Japanese text may contain XML tags and emoji, which should be preserved in the output.</req></requirements>"},
		},
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens:   &maxTokens,
		SystemInstruction: systemInstruction,
	}

	start := time.Now()
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-lite", genai.Text("<text_to_translate>" + targetText + "</text_to_translate>"), config)
	if err != nil {
		log.Fatal(err)
	}
	printResponse(result, time.Since(start))
}

func printResponse(resp *genai.GenerateContentResponse, apiCallDuration time.Duration) {
	fmt.Println("==== Output ====")
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			fmt.Print(part.Text)
		}
	}
	fmt.Println("\n================")
	fmt.Fprintln(os.Stderr, "✓ API call time: ", apiCallDuration)
	fmt.Fprintln(os.Stderr, "✓ Model version: ", resp.ModelVersion)
	fmt.Fprintln(os.Stderr, "✓ Total token count: ", resp.UsageMetadata.TotalTokenCount)
}
