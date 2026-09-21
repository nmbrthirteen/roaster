package metric

import (
	"reflect"
	"testing"

	"github.com/upgaming/roaster/internal/github"
)

const readme = `# Hi there

I'm always ready to go the extra mile. Let's go!

![Rust](https://img.shields.io/badge/Rust-000000?style=for-the-badge&logo=rust)
![Haskell](https://img.shields.io/badge/Haskell-5e5086?logo=haskell&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2CA5E0?logo=docker)

[![My Skills](https://skillicons.dev/icons?i=js,ts,react,kubernetes)](https://skillicons.dev)

<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/python/python-original.svg" />
`

func TestBadgesAreCountedWhereverTheyComeFrom(t *testing.T) {
	// Three shields, four skill icons, one devicon.
	if got := Badges(readme); got != 8 {
		t.Errorf("got %d badges, want 8", got)
	}
}

func TestClaimsComeFromBadgesNotFromProse(t *testing.T) {
	got := Claimed(readme)
	want := []string{"Haskell", "JavaScript", "Python", "Rust", "TypeScript"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, lang := range got {
		if lang == "Go" {
			t.Errorf(`"let's go" in a sentence was read as a claim to write Go`)
		}
	}
}

func TestFrameworksAreNotLanguages(t *testing.T) {
	for _, lang := range Claimed(readme) {
		if lang == "Docker" || lang == "React" || lang == "Kubernetes" {
			t.Errorf("%s has no GitHub language to be checked against", lang)
		}
	}
}

func TestUnusedIsWhatNoRepositoryBacksUp(t *testing.T) {
	repos := []github.Repo{
		{Name: "a", Language: "JavaScript"},
		{Name: "b", Language: "JavaScript"},
		{Name: "c", Language: "TypeScript"},
		{Name: "d"},
	}
	got := Unused(Claimed(readme), repos)
	want := []string{"Haskell", "Python", "Rust"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLanguagesIsTheSplitLargestFirst(t *testing.T) {
	repos := []github.Repo{
		{Language: "JavaScript"}, {Language: "JavaScript"}, {Language: "JavaScript"},
		{Language: "Go"}, {Language: ""},
	}
	got := Languages(repos)
	want := []Share{{"JavaScript", 75}, {"Go", 25}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnEmptyReadmeClaimsNothing(t *testing.T) {
	if got := Claimed(""); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	if got := Badges(""); got != 0 {
		t.Errorf("got %d", got)
	}
}
