package docs

import (
	"html"
	"regexp"
	"strings"
)

const (
	siteOfDefold        = "https://defold.com"
	anchorOfTheSamePage = "#"
	codeFence           = "```"
	listItem            = "- "
)

var (
	tagPattern        = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	hrefPattern       = regexp.MustCompile(`href="([^"]*)"`)
	blankLinesPattern = regexp.MustCompile(`\n{3,}`)
	trailingSpaces    = regexp.MustCompile(`[ \t]+\n`)
)

func Markdown(markup string) string {
	var written strings.Builder
	var links []string
	insidePre, listDepth := false, 0
	position := 0
	text := func(raw string) {
		unescaped := html.UnescapeString(raw)
		if !insidePre {
			unescaped = strings.ReplaceAll(unescaped, "\n", " ")
		}
		written.WriteString(unescaped)
	}
	for _, found := range tagPattern.FindAllStringSubmatchIndex(markup, -1) {
		text(markup[position:found[0]])
		position = found[1]
		isClosing := markup[found[2]:found[3]] == "/"
		name := strings.ToLower(markup[found[4]:found[5]])
		attributes := markup[found[6]:found[7]]
		switch name {
		case "pre":
			if isClosing {
				insidePre = false
				trimmed := strings.TrimRight(written.String(), "\n")
				written.Reset()
				written.WriteString(trimmed + "\n```\n\n")
			} else {
				insidePre = true
				written.WriteString("\n\n```lua\n")
			}
		case "code":
			if !insidePre {
				written.WriteString("`")
			}
		case "p", "div", "h1", "h2", "h3", "h4", "dl", "table", "tr":
			if !insidePre {
				written.WriteString("\n\n")
			}
		case "br":
			written.WriteString("\n")
		case "ul", "ol":
			if isClosing {
				listDepth--
			} else {
				listDepth++
			}
			written.WriteString("\n")
			if listDepth <= 0 {
				written.WriteString("\n")
			}
		case "li", "dt":
			if !isClosing {
				written.WriteString("\n" + strings.Repeat("  ", max(listDepth-1, 0)) + "- ")
			}
		case "dd", "td", "th":
			if !isClosing {
				written.WriteString(" ")
			}
		case "strong", "b":
			written.WriteString("**")
		case "em", "i":
			written.WriteString("*")
		case "a":
			switch {
			case !isClosing:
				target := ""
				if href := hrefPattern.FindStringSubmatch(attributes); href != nil && !strings.HasPrefix(href[1], anchorOfTheSamePage) {
					target = absolute(html.UnescapeString(href[1]))
					written.WriteString("[")
				}
				links = append(links, target)
			case len(links) > 0:
				if target := links[len(links)-1]; target != "" {
					written.WriteString("](" + target + ")")
				}
				links = links[:len(links)-1]
			}
		}
	}
	text(markup[position:])

	cleaned := trailingSpaces.ReplaceAllString(withoutStrayIndentation(written.String()), "\n")
	cleaned = blankLinesPattern.ReplaceAllString(cleaned, "\n\n")
	return strings.TrimSpace(cleaned)
}

func withoutStrayIndentation(text string) string {
	lines := strings.Split(text, "\n")
	insideFence := false
	for index, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, codeFence) {
			insideFence = !insideFence
			lines[index] = trimmed
			continue
		}
		if !insideFence && !strings.HasPrefix(trimmed, listItem) {
			lines[index] = trimmed
		}
	}
	return strings.Join(lines, "\n")
}

func absolute(link string) string {
	if strings.HasPrefix(link, "/") {
		return siteOfDefold + link
	}
	return link
}

func OneLine(markup string) string {
	return strings.Join(strings.Fields(Markdown(markup)), " ")
}
