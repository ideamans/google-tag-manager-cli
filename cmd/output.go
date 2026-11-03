package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// renderJSON は値を整形済み JSON で標準出力に書き出す（既定の出力）。
func renderJSON(v any) error { return writeJSON(os.Stdout, v) }

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// addFormatFlag は --format json|table を足す。
func addFormatFlag(cmd *cobra.Command, format *string) {
	cmd.Flags().StringVar(format, "format", "json", "Output format: json or table")
}

func validateFormat(format string) error {
	switch format {
	case "json", "table":
		return nil
	}
	return fmt.Errorf("--format must be json or table (given: %s)", format)
}

// printTable はヘッダーと行を桁揃えで出す。
func printTable(header []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	_ = w.Flush()
	fmt.Printf("\n%d rows\n", len(rows))
}

// toMap は API の型を JSON のキーで引ける map にする（表の列や部分更新に使う）。
func toMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

// readBody は --file のパス（- なら標準入力）から JSON を読む。
func readBody(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return b, nil
}

// requireYes は取り消しにくい操作に --yes を求める。
func requireYes(yes bool, what string) error {
	if yes {
		return nil
	}
	return fmt.Errorf("%s. Re-run with --yes to confirm", what)
}
