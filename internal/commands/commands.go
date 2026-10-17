// Package commands defines the fixed slash commands of the chat composer
// (/grill-me, /write-pages, ...). A command is plain prompt text: it is
// expanded before the message is delivered to the agent, so it works with any
// agent and changes nothing else.
package commands

import "strings"

type Command struct {
	Name        string `json:"name"`        // without the slash
	Description string `json:"description"` // shown in the composer menu
	Usage       string `json:"usage"`       // argument hint, may be empty
	Prompt      string `json:"-"`           // {args} is replaced by what the user typed after the command
}

// Marker starts every expanded prompt, so a prompt that comes back through
// the agent's own transcript can be recognised as AICW's and not shown twice.
const Marker = "[AICW /"

const toolHint = `Use the AICW tools to save things: via MCP (list_pages, read_page, write_page, append_page, write_note, read_conversation) or, if you have no MCP tools, the shell command ` + "`aicw tool <name> '<json>'`" + `.`

var all = []Command{
	{
		Name: "grill-me", Usage: "[topic]", Description: "The AI interviews you, one question at a time, until it has everything it needs",
		Prompt: `Interview me before doing any work{topic}.

Rules:
- Ask exactly ONE question per message and wait for my answer. Never list several questions.
- Start with the most important unknown. Build on my previous answers; don't repeat what I already said.
- Look at the project (files, notes, pages) first and don't ask what you can find out yourself.
- Offer 2-4 concrete options when it helps me answer fast, plus your recommendation.
- Push back if an answer is vague, contradictory or risky, and ask a sharper follow-up.
- Keep going until you are confident you have every detail (goals, constraints, users, edge cases, priorities, definition of done). Then stop asking and reply with: a summary of everything I told you, the open risks, and ask whether I want it saved with /write-pages.

Ask your first question now.`,
	},
	{
		Name: "write-pages", Usage: "[what to focus on]", Description: "The AI saves everything from this conversation into project pages by itself",
		Prompt: `Write down everything worth keeping from this conversation as project pages, now, without asking me anything first{focus}.

- Read the existing pages first (list_pages / read_page) and update them instead of duplicating; use append_page for additions.
- Split by topic: a short overview page, then one page per major topic (decisions, requirements, plan, open questions, reference). Each page: a clear title, a one-sentence summary, then well-structured Markdown with headings and lists.
- Include concrete details exactly as discussed: names, numbers, decisions and why, constraints, file paths, commands, and unresolved questions. Don't invent facts that weren't discussed; mark guesses as such.
- Reference images, audio or video I attached with ![caption](/absolute/path) so AICW stores them.
- Never put secrets, tokens or credentials on a page.
` + toolHint + `
When finished, list the pages you created or changed (title and one line each).`,
	},
	{
		Name: "summarize", Usage: "", Description: "Summarise this conversation into a short note the AI will read next session",
		Prompt: `Summarise this conversation: what we decided, what was done, what is still open, and the exact next steps. Save it with write_note under the name "session-summary" (replace the previous one, but keep any still-valid facts from it). ` + toolHint + ` Then show me the summary.`,
	},
	{
		Name: "plan", Usage: "<goal>", Description: "Ask what's unclear, then write a step-by-step plan before touching anything",
		Prompt: `Plan this before doing anything{goal}. Don't change any files yet.

First read the relevant parts of the project. If something essential is unclear, ask me up to 3 short questions, one at a time. Then give a numbered plan: the steps, the files each touches, how we will check each step worked, and the risks. Wait for my approval before starting.`,
	},
	{
		Name: "review", Usage: "[what to review]", Description: "Review recent changes for bugs, risks and missing tests",
		Prompt: `Review the recent changes in this project{what}. Read the diff (git status / git diff) and the code around it. Report only real problems, most serious first: bugs, security issues, broken edge cases, missing tests. For each: where it is, why it is wrong, and the fix. If it's clean, say so. Don't change files unless I ask.`,
	},
}

func All() []Command { return append([]Command(nil), all...) }

// Expand turns "/name args" into the agent prompt. ok is false when text is
// not one of the fixed commands, in which case it must be sent unchanged.
func Expand(text string) (prompt string, cmd Command, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "/") {
		return "", Command{}, false
	}
	name, args, _ := strings.Cut(t[1:], " ")
	if i := strings.IndexAny(name, "\n\t"); i >= 0 { // "/plan\nfoo"
		name, args = name[:i], name[i+1:]+" "+args
	}
	args = strings.TrimSpace(args)
	for _, c := range all {
		if !strings.EqualFold(c.Name, name) {
			continue
		}
		p := c.Prompt
		for _, k := range []string{"{topic}", "{focus}", "{goal}", "{what}"} {
			rep := ""
			if args != "" {
				switch k {
				case "{topic}":
					rep = " about: " + args
				case "{focus}":
					rep = ". Focus on: " + args
				case "{goal}":
					rep = ": " + args
				case "{what}":
					rep = " (" + args + ")"
				}
			}
			p = strings.ReplaceAll(p, k, rep)
		}
		return Marker + c.Name + "]\n" + p, c, true
	}
	return "", Command{}, false
}
