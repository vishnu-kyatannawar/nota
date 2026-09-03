// Package mdnote parses and serialises Nota's note format.
//
// The format is ordinary markdown, chosen so a note stays readable and editable
// without this application. Action items are GitHub-style task list entries;
// labels (#label) and logged time ([hh:mm]) stay visible inline where a person
// would write them anyway, and only machine bookkeeping — stable ids and exact
// timestamps — hides in an HTML comment that markdown renderers do not display.
//
// The parser is deliberately line-based rather than built on a markdown AST.
// Round-tripping a note without changing it is this package's entire promise,
// and an AST would have to reconstruct formatting it never recorded. Working a
// line at a time means the bytes we did not interpret are the bytes we write
// back. Item text is likewise stored verbatim: Labels and Minutes are derived
// views over that text, never a replacement for it.
package mdnote

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// bodyIndent is how far an item's body is indented past the item's own bullet,
// which lines it up under the text after "- [ ] ".
const bodyIndent = 6

// indentUnit is how many spaces one level of item nesting uses.
const indentUnit = 2

var (
	itemLine   = regexp.MustCompile(`^([ \t]*)- \[([ xX])\] ?(.*)$`)
	metaBlock  = regexp.MustCompile(`\s*<!--n ([^>]*)-->\s*$`)
	labelTok   = regexp.MustCompile(`#([\p{L}\p{N}][\p{L}\p{N}_/-]*)`)
	timeTok    = regexp.MustCompile(`\[(\d{2}):([0-5]\d)\]`)
	fenceLine  = regexp.MustCompile("^\\s*(```|~~~)")
	durationRe = regexp.MustCompile(`^(\d{2,}):([0-5]\d)$`)
)

// Note is a single markdown file: its frontmatter, its action items, and any
// free-form markdown that follows them.
type Note struct {
	ID      string   `yaml:"id,omitempty"`
	Type    string   `yaml:"type,omitempty"`
	Date    string   `yaml:"date,omitempty"`
	Hours   string   `yaml:"hours,omitempty"`
	DayType string   `yaml:"daytype,omitempty"`
	Labels  []string `yaml:"labels,omitempty"`

	// Items are the action items, in file order and including nested ones.
	Items []Item `yaml:"-"`
	// Body is the markdown after the last action item, verbatim.
	Body string `yaml:"-"`
	// HadFrontmatter records whether the source had a frontmatter block, so a
	// note without one does not gain one just by being saved.
	HadFrontmatter bool `yaml:"-"`
}

// Item is one action item.
type Item struct {
	// Text is the visible text, including any #labels and [hh:mm] token. It is
	// stored exactly as written so serialising cannot reorder or drop anything.
	Text string
	Done bool
	// Depth is the nesting level; 0 is top level.
	Depth int
	// Body holds the item's own markdown lines with the common indent removed.
	Body []string

	ID        string // stable across rollover
	CreatedAt string // "hh:mm" — when the item was added
	DoneAt    string // "hh:mm" — when it was ticked
	From      string // "yyyy-mm-dd" the item was first created, once carried
	Carried   int    // how many days it has rolled over
	Recurring string // id of the recurring template that seeded it
}

// Parse reads a note. Carriage returns are dropped so a file edited on Windows
// parses identically to one edited elsewhere.
func Parse(src string) (*Note, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")

	note := &Note{}
	rest := src

	if strings.HasPrefix(src, "---\n") {
		end := strings.Index(src[4:], "\n---\n")
		closing := 5
		if end < 0 {
			// A frontmatter block may also close at end of file with no trailing body.
			if strings.HasSuffix(src, "\n---\n") {
				end = len(src) - 4 - 5
				closing = 5
			} else if strings.HasSuffix(src, "\n---") {
				end = len(src) - 4 - 4
				closing = 4
			}
		}
		if end >= 0 {
			front := src[4 : 4+end]
			if err := yaml.Unmarshal([]byte(front), note); err != nil {
				return nil, fmt.Errorf("parsing frontmatter: %w", err)
			}
			note.HadFrontmatter = true
			rest = src[4+end+closing:]
		}
	}

	rest = strings.TrimLeft(rest, "\n")
	note.Items, note.Body = parseItems(rest)
	return note, nil
}

// parseItems walks the body a line at a time, collecting action items with their
// bodies and returning whatever trailing markdown is left over.
func parseItems(src string) ([]Item, string) {
	if src == "" {
		return nil, ""
	}
	lines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")

	var (
		items    []Item
		current  *Item
		body     []string
		inFence  bool
		curIndnt int
	)

	flush := func() {
		if current == nil {
			return
		}
		current.Body = trimBlankEdges(body)
		items = append(items, *current)
		current, body = nil, nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// A fence inside an item body swallows everything until it closes, so
		// checklist syntax used as example content is never read as an item.
		if current != nil && fenceLine.MatchString(line) && indentOf(line) >= curIndnt+indentUnit {
			inFence = !inFence
			body = append(body, dedent(line, curIndnt+bodyIndent))
			continue
		}
		if inFence {
			body = append(body, dedent(line, curIndnt+bodyIndent))
			continue
		}

		if m := itemLine.FindStringSubmatch(line); m != nil {
			flush()
			indent := len(strings.ReplaceAll(m[1], "\t", strings.Repeat(" ", indentUnit)))
			text, meta := splitMeta(m[3])
			it := Item{
				Text:  strings.TrimRight(text, " \t"),
				Done:  m[2] == "x" || m[2] == "X",
				Depth: indent / indentUnit,
			}
			applyMeta(&it, meta)
			current, curIndnt = &it, indent
			continue
		}

		if current != nil && (strings.TrimSpace(line) == "" || indentOf(line) >= curIndnt+indentUnit) {
			body = append(body, dedent(line, curIndnt+bodyIndent))
			continue
		}

		// An unindented, non-item line ends the item list; the remainder is body.
		flush()
		return items, strings.Join(lines[i:], "\n") + "\n"
	}

	flush()
	return items, ""
}

// splitMeta separates the visible text from the trailing <!--n ...--> comment.
func splitMeta(s string) (text, meta string) {
	m := metaBlock.FindStringSubmatchIndex(s)
	if m == nil {
		return s, ""
	}
	return s[:m[0]], strings.TrimSpace(s[m[2]:m[3]])
}

func applyMeta(it *Item, meta string) {
	for _, field := range strings.Fields(meta) {
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		switch key {
		case "id":
			it.ID = value
		case "t":
			it.CreatedAt = value
		case "done":
			it.DoneAt = value
		case "from":
			it.From = value
		case "rec":
			it.Recurring = value
		case "carried":
			if n, err := strconv.Atoi(value); err == nil {
				it.Carried = n
			}
		}
	}
	// "t" and "done" are clock times, so a colon inside them was split above.
	// Re-join the minute part that Cut removed.
	rejoinClock(&it.CreatedAt, meta, "t")
	rejoinClock(&it.DoneAt, meta, "done")
}

// rejoinClock repairs an hh:mm value, since splitting a field on its first colon
// leaves only the hour behind.
func rejoinClock(dst *string, meta, key string) {
	re := regexp.MustCompile(`\b` + key + `:(\d{2}:[0-5]\d)\b`)
	if m := re.FindStringSubmatch(meta); m != nil {
		*dst = m[1]
	}
}

// Serialize writes a note back to markdown. Output is canonical: parsing and
// serialising again yields exactly the same bytes.
func Serialize(n *Note) string {
	var b strings.Builder

	if n.HadFrontmatter || n.ID != "" || n.Type != "" || n.Date != "" || n.Hours != "" || n.DayType != "" || len(n.Labels) > 0 {
		b.WriteString("---\n")
		b.WriteString(frontmatter(n))
		b.WriteString("---\n")
		if len(n.Items) > 0 || n.Body != "" {
			b.WriteString("\n")
		}
	}

	for i, it := range n.Items {
		indent := strings.Repeat(" ", it.Depth*indentUnit)
		mark := " "
		if it.Done {
			mark = "x"
		}
		b.WriteString(indent + "- [" + mark + "] " + it.Text)
		if meta := formatMeta(it); meta != "" {
			b.WriteString(" " + meta)
		}
		b.WriteString("\n")

		if len(it.Body) > 0 {
			pad := strings.Repeat(" ", it.Depth*indentUnit+bodyIndent)
			for _, line := range it.Body {
				if line == "" {
					b.WriteString("\n")
					continue
				}
				b.WriteString(pad + line + "\n")
			}
			// A body reads as a block, so it is followed by a blank line unless
			// the note ends here.
			if i < len(n.Items)-1 || n.Body != "" {
				b.WriteString("\n")
			}
		}
	}

	if n.Body != "" {
		// Exactly one blank line separates the item list from the trailing prose,
		// however the source happened to space them.
		if len(n.Items) > 0 && !strings.HasSuffix(b.String(), "\n\n") {
			b.WriteString("\n")
		}
		b.WriteString(n.Body)
	}
	return b.String()
}

// frontmatter emits the known keys in a fixed order so saving never reshuffles them.
func frontmatter(n *Note) string {
	var b strings.Builder
	if n.ID != "" {
		b.WriteString("id: " + n.ID + "\n")
	}
	if n.Type != "" {
		b.WriteString("type: " + n.Type + "\n")
	}
	if n.Date != "" {
		b.WriteString("date: " + n.Date + "\n")
	}
	if n.Hours != "" {
		// Quoted so YAML reads "09:00" as a string rather than a sexagesimal number.
		b.WriteString(`hours: "` + n.Hours + "\"\n")
	}
	if n.DayType != "" {
		b.WriteString("daytype: " + n.DayType + "\n")
	}
	if len(n.Labels) > 0 {
		b.WriteString("labels: [" + strings.Join(n.Labels, ", ") + "]\n")
	}
	return b.String()
}

func formatMeta(it Item) string {
	var parts []string
	if it.ID != "" {
		parts = append(parts, "id:"+it.ID)
	}
	if it.CreatedAt != "" {
		parts = append(parts, "t:"+it.CreatedAt)
	}
	if it.DoneAt != "" {
		parts = append(parts, "done:"+it.DoneAt)
	}
	if it.From != "" {
		parts = append(parts, "from:"+it.From)
	}
	if it.Carried > 0 {
		parts = append(parts, "carried:"+strconv.Itoa(it.Carried))
	}
	if it.Recurring != "" {
		parts = append(parts, "rec:"+it.Recurring)
	}
	if len(parts) == 0 {
		return ""
	}
	return "<!--n " + strings.Join(parts, " ") + "-->"
}

// Labels returns the #labels written in the item's text, in order of appearance.
func (it Item) Labels() []string {
	var out []string
	for _, m := range labelTok.FindAllStringSubmatch(stripFences(it.Text), -1) {
		out = append(out, m[1])
	}
	return out
}

// Minutes returns the logged time from the item's [hh:mm] token, or zero.
func (it Item) Minutes() int {
	m := timeTok.FindStringSubmatch(it.Text)
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	return h*60 + mins
}

// SetMinutes writes the logged time into the item's text, replacing an existing
// token in place so the rest of the line keeps its wording and order. Zero
// removes the token entirely rather than writing a meaningless [00:00].
func (it *Item) SetMinutes(minutes int) {
	if minutes <= 0 {
		it.Text = strings.TrimSpace(collapseSpaces(timeTok.ReplaceAllString(it.Text, "")))
		return
	}
	token := "[" + FormatDuration(minutes) + "]"
	if timeTok.MatchString(it.Text) {
		it.Text = timeTok.ReplaceAllString(it.Text, token)
		return
	}
	it.Text = strings.TrimRight(it.Text, " ") + " " + token
}

// AllLabels returns every label on the note and its items, sorted and deduplicated.
func (n *Note) AllLabels() []string {
	seen := map[string]bool{}
	for _, l := range n.Labels {
		seen[l] = true
	}
	for _, it := range n.Items {
		for _, l := range it.Labels() {
			seen[l] = true
		}
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// TotalMinutes sums the time logged against every item.
func (n *Note) TotalMinutes() int {
	total := 0
	for _, it := range n.Items {
		total += it.Minutes()
	}
	return total
}

// ParseDuration reads an "hh:mm" duration into minutes. Hours are zero padded to
// at least two digits, which is the format the user writes.
func ParseDuration(s string) (int, bool) {
	m := durationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	h, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	mins, _ := strconv.Atoi(m[2])
	return h*60 + mins, true
}

// FormatDuration renders minutes as "hh:mm".
func FormatDuration(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += indentUnit
		default:
			return n
		}
	}
	return n
}

// dedent removes up to width leading spaces, leaving deeper indentation intact so
// nested code keeps its shape.
func dedent(line string, width int) string {
	for i := 0; i < width; i++ {
		if strings.HasPrefix(line, " ") {
			line = line[1:]
			continue
		}
		break
	}
	return strings.TrimRight(line, " \t")
}

func trimBlankEdges(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// stripFences removes inline code spans so a # inside code is not read as a label.
func stripFences(s string) string {
	var b strings.Builder
	inCode := false
	for _, r := range s {
		if r == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func collapseSpaces(s string) string {
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
