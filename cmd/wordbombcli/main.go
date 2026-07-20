// Command wordbombcli is the GUI-less interface to the Word Bomb Tool: word
// suggestions and definitions via the Datamuse API. It is the Go port of cli.py.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/datamuse"
	"github.com/mphpmaster/word-bomb-tool-go/internal/suggest"
)

var searchAliases = map[string]string{
	"starts-with": "Starts With",
	"starts":      "Starts With",
	"sw":          "Starts With",
	"ends-with":   "Ends With",
	"ends":        "Ends With",
	"ew":          "Ends With",
	"contains":    "Contains",
	"c":           "Contains",
	"rhymes":      "Rhymes",
	"r":           "Rhymes",
	"related":     "Related Words",
	"related-words": "Related Words",
	"rel":         "Related Words",
}

var sortAliases = map[string]string{
	"shortest":  "Shortest",
	"s":         "Shortest",
	"longest":   "Longest",
	"l":         "Longest",
	"random":    "Random",
	"rand":      "Random",
	"frequency": "Frequency",
	"freq":      "Frequency",
	"f":         "Frequency",
}

func resolveMode(alias string, mapping map[string]string, canonical []string, label string) (string, error) {
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(alias)), "_", "-")
	if v, ok := mapping[key]; ok {
		return v, nil
	}
	for _, m := range canonical {
		low := strings.ToLower(m)
		if low == key || strings.ReplaceAll(low, " ", "-") == key {
			return m, nil
		}
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "", fmt.Errorf("unknown %s %q; try one of: %s", label, alias, strings.Join(keys, ", "))
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	if len(argv) == 0 {
		usage()
		return 2
	}

	// A leading -v/--verbose global flag is accepted before the subcommand.
	for len(argv) > 0 && (argv[0] == "-v" || argv[0] == "--verbose") {
		argv = argv[1:]
	}
	if len(argv) == 0 {
		usage()
		return 2
	}

	cmd := argv[0]
	rest := argv[1:]
	switch cmd {
	case "suggest":
		return cmdSuggest(rest)
	case "define":
		return cmdDefine(rest)
	case "modes":
		return cmdModes()
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "wbt — Word Bomb Tool CLI (word suggestions and definitions via Datamuse)")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	fmt.Fprintln(os.Stderr, "  suggest LETTERS [--mode M] [--sort S] [--limit N] [--json] [--pretty-json]")
	fmt.Fprintln(os.Stderr, "  define WORD [--json] [--pretty-json]")
	fmt.Fprintln(os.Stderr, "  modes")
}

func cmdSuggest(argv []string) int {
	fs := flag.NewFlagSet("suggest", flag.ContinueOnError)
	mode := fs.String("mode", "starts-with", "search mode")
	fs.StringVar(mode, "m", "starts-with", "search mode (shorthand)")
	sortMode := fs.String("sort", "shortest", "sort mode")
	fs.StringVar(sortMode, "s", "shortest", "sort mode (shorthand)")
	limit := fs.Int("limit", config.MaxSuggestionsDisplay, "max words to print")
	fs.IntVar(limit, "n", config.MaxSuggestionsDisplay, "max words to print (shorthand)")
	asJSON := fs.Bool("json", false, "print JSON to stdout")
	prettyJSON := fs.Bool("pretty-json", false, "pretty-print JSON (only with --json)")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: missing LETTERS argument")
		return 2
	}

	searchMode, err := resolveMode(*mode, searchAliases, config.SearchModes, "search mode")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	sMode, err := resolveMode(*sortMode, sortAliases, config.SortModes, "sort mode")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	lim := *limit
	if lim < 1 {
		lim = 1
	}
	if lim > config.MaxSuggestionsDisplay {
		lim = config.MaxSuggestionsDisplay
	}

	letters := strings.TrimSpace(fs.Arg(0))
	if letters == "" {
		fmt.Fprintln(os.Stderr, "error: letters must not be empty")
		return 2
	}

	client := datamuse.New()
	raw := client.Suggestions(letters, searchMode)
	words := suggest.Sort(raw, sMode)
	if len(words) > lim {
		words = words[:lim]
	}

	if *asJSON {
		printJSON(map[string]interface{}{
			"letters":     letters,
			"search_mode": searchMode,
			"sort_mode":   sMode,
			"api_status":  client.Status(),
			"words":       words,
		}, *prettyJSON)
		return 0
	}

	fmt.Printf("search: %s  sort: %s  api: %s\n", searchMode, sMode, client.Status())
	if len(words) == 0 {
		fmt.Println("(no words)")
		return 0
	}
	for i, w := range words {
		fmt.Printf("%4d  %s\n", i+1, w)
	}
	return 0
}

func cmdDefine(argv []string) int {
	fs := flag.NewFlagSet("define", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON to stdout")
	prettyJSON := fs.Bool("pretty-json", false, "pretty-print JSON (only with --json)")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: missing WORD argument")
		return 2
	}
	word := strings.TrimSpace(fs.Arg(0))
	if word == "" {
		fmt.Fprintln(os.Stderr, "error: word must not be empty")
		return 2
	}

	client := datamuse.New()
	defs := client.Definitions(word)

	if *asJSON {
		printJSON(map[string]interface{}{
			"word":        word,
			"api_status":  client.Status(),
			"definitions": defs,
		}, *prettyJSON)
		return 0
	}

	fmt.Printf("word: %s  api: %s\n", word, client.Status())
	if len(defs) == 0 {
		fmt.Println("(no definitions)")
		return 0
	}
	for i, d := range defs {
		fmt.Printf("%d. %s\n", i+1, d)
	}
	return 0
}

func cmdModes() int {
	fmt.Println("Search modes (use with suggest --mode):")
	for _, m := range config.SearchModes {
		fmt.Printf("  - %s\n", m)
	}
	fmt.Println("\nSort modes (use with suggest --sort):")
	for _, m := range config.SortModes {
		fmt.Printf("  - %s\n", m)
	}
	fmt.Println("\nAliases examples: starts-with, ends-with, contains, rhymes, related")
	fmt.Println("                  shortest, longest, random, frequency")
	return 0
}

func printJSON(v interface{}, pretty bool) {
	var (
		data []byte
		err  error
	)
	if pretty {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	fmt.Println(string(data))
}
