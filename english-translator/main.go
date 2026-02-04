package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

)

// コマンドライン引数を解析し、モデル名、初期化フラグ、タスク定義、入力テキストを返す
// ただしinitがtrueの場合はテキストは不要
func parseArgs() (modelName string, thinkingFlag bool, initFlag bool, task TaskDefinition, inputText string, err error) {
	defaultTask, _ := getTaskDefinition("translate")

	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagSet.SetOutput(flag.CommandLine.Output())
	flagSet.StringVar(&modelName, "model", "gemini-2.5-flash", "モデル名を指定します")
	var taskName string
	flagSet.StringVar(&taskName, "task", "", "タスク名を指定します (必須)")
	flagSet.BoolVar(&thinkingFlag, "think", false, "思考プロセスを有効にします")
	flagSet.BoolVar(&initFlag, "init", false, "対話形式で設定を初期化します")

	// カスタムUsage関数を設定（タスク指定ルールを追加）
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), "Usage: %s [options] <入力テキスト>\n\n", os.Args[0])
		fmt.Fprintf(flagSet.Output(), "Example: %s --task translate \"翻訳したいテキスト\"\n\n", os.Args[0])
		fmt.Fprintf(flagSet.Output(), "Init only: %s -init\n\n", os.Args[0])
		fmt.Fprintf(flagSet.Output(), "Options:\n")
		flagSet.PrintDefaults()
		fmt.Fprintf(flagSet.Output(), "\nTasks:\n%s\n", taskUsageLines())
	}

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return "", false, false, defaultTask, "", err
	}

	// -initフラグが設定されている場合は、タスクとテキストは不要
	if initFlag {
		return modelName, false, initFlag, defaultTask, "", nil
	}

	if strings.TrimSpace(taskName) == "" {
		flagSet.Usage()
		return "", false, false, defaultTask, "", fmt.Errorf("タスク名を --task で指定してください")
	}

	parsedTask, ok := getTaskDefinition(taskName)
	if !ok {
		flagSet.Usage()
		return "", false, false, defaultTask, "", fmt.Errorf("無効なタスク名が指定されています (-task): %s", taskName)
	}

	args := flagSet.Args()
	if len(args) < 1 {
		flagSet.Usage()
		return "", false, false, defaultTask, "", fmt.Errorf("入力テキストが指定されていません")
	}

	inputText = strings.Join(args, " ")
	return modelName, thinkingFlag, initFlag, parsedTask, inputText, nil
}

func main() {
	// コマンドライン引数の解析と検証
	modelName, thinkingFlag, initFlag, task, inputText, err := parseArgs()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
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
	llmReqConfig, genaiConfig := createLLMConfigs(task, modelName, inputText, thinkingFlag)

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
	printMetadata(metadata, apiMethod, task.Name)
}
