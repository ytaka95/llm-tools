package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

)

// コマンドライン引数を解析し、モデル名、初期化フラグ、翻訳対象テキストを返す
// ただしinitがtrueの場合はテキストは不要
func parseArgs() (modelName string, thinkingFlag bool, initFlag bool, targetText string, err error) {
	flag.StringVar(&modelName, "model", "gemini-2.5-flash", "モデル名を指定します")
	flag.BoolVar(&thinkingFlag, "think", false, "思考プロセスを有効にします")
	flag.BoolVar(&initFlag, "init", false, "対話形式で設定を初期化します")

	// カスタムUsage関数を設定（位置引数の説明のみ追加）
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <翻訳したい日本語テキスト>\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// -initフラグが設定されている場合は、テキスト引数は不要
	if initFlag {
		return modelName, false, initFlag, "", nil
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return "", false, false, "", fmt.Errorf("翻訳対象テキストが指定されていません")
	}

	targetText = args[0]
	return modelName, thinkingFlag, initFlag, targetText, nil
}

func main() {
	// コマンドライン引数の解析と検証
	modelName, thinkingFlag, initFlag, targetText, err := parseArgs()
	if err != nil {
		os.Exit(1)
	}

	// -initフラグが指定された場合は対話型セットアップを実行して終了
	if initFlag {
		fmt.Println("設定を初期化します...")
		_, err := setupInteractive()
		if err != nil {
			fmt.Fprintf(os.Stderr, "設定の初期化中にエラーが発生しました: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("設定の初期化が完了しました。")
		return
	}

	// 設定の読み込みまたは対話型セットアップ
	settings, err := loadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "設定の読み込み中にエラーが発生しました: %v\n", err)
		os.Exit(1)
	}

	// 設定ファイルが存在しない場合は対話型セットアップを実行
	if settings == nil {
		settings, err = setupInteractive()
		if err != nil {
			fmt.Fprintf(os.Stderr, "設定のセットアップ中にエラーが発生しました: %v\n", err)
			os.Exit(1)
		}
	}

	// クライアントの初期化
	ctx := context.Background()
	client, apiMethod, err := initClient(ctx, settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 出力処理用のチャネルとgoroutineの設定
	outputChan := make(chan string, 100)
	done := make(chan bool)

	go func() {
		charLengthPerStep := 5
		timePerChar := 15 * time.Millisecond
		lastTextEndedWithNewline := false

		for text := range outputChan {
			var start = 0
			for start < len(text) {
				end := min(start+charLengthPerStep, len(text))
				fmt.Print(text[start:end])
				start = end
				// 最後のチャンクでなければ待機
				if start < len(text) {
					time.Sleep(timePerChar)
				}
			}

			// 最後のテキストが改行かどうかを記録
			if len(text) > 0 && text[len(text)-1] == '\n' {
				lastTextEndedWithNewline = true
			} else {
				lastTextEndedWithNewline = false
			}
		}

		// 最後のテキストが改行でなければ改行を出力
		if !lastTextEndedWithNewline {
			fmt.Println()
		}

		done <- true
	}()

	// LLMリクエストと生成コンテンツの設定作成
	llmReqConfig, genaiConfig := createLLMConfigs(modelName, targetText, thinkingFlag)

	// ストリーミングAPI呼び出しと結果処理
	metadata, err := streamContent(ctx, client, llmReqConfig, genaiConfig, outputChan)

	// 出力チャネルをクローズし、出力ゴルーチンの終了を待つ
	close(outputChan)
	<-done

	// エラーハンドリング
	if err != nil {
		if !strings.Contains(err.Error(), "見つからないか、generateContentをサポートしていません") {
			log.Fatal(err)
		} else {
			os.Exit(1)
		}
	}

	// メタデータの表示
	printMetadata(metadata, apiMethod)
}
