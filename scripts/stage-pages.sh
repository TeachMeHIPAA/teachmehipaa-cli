#!/bin/sh
# Copies the documentation the docs site publishes into one directory for
# Jekyll, the way GitHub Pages builds a repository: README.md becomes the
# home page and links between .md files become links between pages.
#
#   scripts/stage-pages.sh _pages
set -eu
out="${1:?usage: stage-pages.sh <directory>}"

rm -rf "$out"
mkdir -p "$out"
cp -R README.md CHANGELOG.md CONTRIBUTING.md GLOSSARY.md SECURITY.md LICENSE docs examples "$out/"
cp .github/pages/_config.yml "$out/"

# Each page gets front matter naming it after its first heading, and a
# README is its directory's index page. Its body is wrapped in a Liquid raw
# block: Jekyll runs Liquid on every page, and the docs show `{{ … }}` from
# GitHub Actions and Go templates literally.
find "$out" -name '*.md' | while read -r page; do
	title="$(grep -m 1 '^#' "$page" | sed -e 's/^#* *//' -e 's/`//g' -e 's/\\/\\\\/g' -e 's/"/\\"/g')"
	permalink=""
	if [ "$(basename "$page")" = README.md ]; then
		dir="$(dirname "${page#"$out"}")"
		permalink="permalink: ${dir%/}/\n"
	fi
	{
		printf -- "---\ntitle: \"%s\"\n${permalink}---\n{%% raw %%}\n" "$title"
		cat "$page"
		printf '\n{%% endraw %%}\n'
	} >"$page.staged"
	mv "$page.staged" "$page"
done
