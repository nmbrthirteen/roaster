package metric

import (
	"regexp"
	"sort"
	"strings"

	"github.com/upgaming/roaster/internal/github"
)

// A profile README says what someone wants to be seen using. The badges are
// the declaration, so the badges are what is read: prose is not, because
// "let's go" in a sentence is not a claim to write Go, and a receipt that says
// otherwise does not survive the person reading it.
var (
	shieldLabel = regexp.MustCompile(`img\.shields\.io/badge/([A-Za-z0-9.+#%_ ]+?)-`)
	shieldLogo  = regexp.MustCompile(`[?&]logo=([A-Za-z0-9.+#-]+)`)
	skillIcons  = regexp.MustCompile(`skillicons\.dev/icons\?i=([A-Za-z0-9,+#.-]+)`)
	devicon     = regexp.MustCompile(`devicons?/devicon[^\s)"']*/icons/([a-z0-9+#.-]+)/`)
)

// languages maps how badges spell a language to how GitHub names it, so a
// claim can be checked against what the repositories are actually written in.
// Only languages: a framework has no GitHub language to be checked against.
var languages = map[string]string{
	"go": "Go", "golang": "Go",
	"rust":   "Rust",
	"python": "Python", "py": "Python",
	"typescript": "TypeScript", "ts": "TypeScript",
	"javascript": "JavaScript", "js": "JavaScript",
	"java":   "Java",
	"kotlin": "Kotlin", "kt": "Kotlin",
	"swift":     "Swift",
	"c":         "C",
	"cplusplus": "C++", "cpp": "C++", "c++": "C++", "c%2b%2b": "C++",
	"csharp": "C#", "cs": "C#", "c#": "C#", "c%23": "C#",
	"ruby":     "Ruby",
	"php":      "PHP",
	"haskell":  "Haskell",
	"elixir":   "Elixir",
	"scala":    "Scala",
	"dart":     "Dart",
	"lua":      "Lua",
	"julia":    "Julia",
	"zig":      "Zig",
	"ocaml":    "OCaml",
	"clojure":  "Clojure",
	"perl":     "Perl",
	"solidity": "Solidity",
	"r":        "R",
}

// Badges counts the images a README uses to list its skills.
func Badges(readme string) int {
	n := len(shieldLabel.FindAllStringIndex(readme, -1))
	n += len(devicon.FindAllStringIndex(readme, -1))
	for _, m := range skillIcons.FindAllStringSubmatch(readme, -1) {
		n += len(strings.Split(m[1], ","))
	}
	return n
}

// Claimed is every language the README's badges name, as GitHub spells it,
// sorted and without repeats.
func Claimed(readme string) []string {
	seen := map[string]bool{}
	add := func(raw string) {
		if lang, ok := languages[strings.ToLower(strings.TrimSpace(raw))]; ok {
			seen[lang] = true
		}
	}

	for _, m := range shieldLabel.FindAllStringSubmatch(readme, -1) {
		add(m[1])
	}
	for _, m := range shieldLogo.FindAllStringSubmatch(readme, -1) {
		add(m[1])
	}
	for _, m := range skillIcons.FindAllStringSubmatch(readme, -1) {
		for _, name := range strings.Split(m[1], ",") {
			add(name)
		}
	}
	for _, m := range devicon.FindAllStringSubmatch(readme, -1) {
		add(m[1])
	}

	out := make([]string, 0, len(seen))
	for lang := range seen {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

// Unused is the claims no repository backs up: a language on the badges that
// is not the main language of anything the account owns.
func Unused(claimed []string, repos []github.Repo) []string {
	used := map[string]bool{}
	for _, r := range repos {
		if r.Language != "" {
			used[r.Language] = true
		}
	}
	var out []string
	for _, lang := range claimed {
		if !used[lang] {
			out = append(out, lang)
		}
	}
	return out
}

// Share is one language and how much of the account is written in it.
type Share struct {
	Language string
	Percent  int
}

// Languages is the split of repositories by main language, largest first. A
// repository with no language detected is left out rather than counted as
// nothing.
func Languages(repos []github.Repo) []Share {
	counts := map[string]int{}
	total := 0
	for _, r := range repos {
		if r.Language == "" {
			continue
		}
		counts[r.Language]++
		total++
	}

	out := make([]Share, 0, len(counts))
	for lang, n := range counts {
		out = append(out, Share{Language: lang, Percent: percent(n, total)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Percent != out[j].Percent {
			return out[i].Percent > out[j].Percent
		}
		return out[i].Language < out[j].Language
	})
	return out
}
