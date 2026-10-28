---
name: asd-ste100
description: This skill should be used when the user asks to "write this in STE", "use Simplified Technical English", "ASD-STE100", "controlled English", "write this for non-native English readers", "make these instructions unambiguous", or asks for procedures, warnings, maintenance steps, runbooks, or agent-facing instructions that must not be misread. Writes and audits text against the ASD-STE100 rules and controlled vocabulary. Not for marketing copy, essays, or the user's personal prose voice.
version: 0.1.0
---

# Simplified Technical English (ASD-STE100)

Write so that a tired reader with weak English, at 3am, with a wrench in one hand, cannot
misread the sentence. ASD-STE100 gets there by two means at once: 53 writing rules in 9
sections, and a controlled dictionary of about 900 approved words in which each word
carries exactly one meaning and one part of speech. `close` means to shut a valve. It
never means near.

The governing principle is closed-world: **everything not explicitly permitted is
prohibited.** A word you cannot justify from the dictionary or from the technical-name
and technical-verb allowances is not available to you.

## When to apply it

Apply on request, and offer it for procedures, safety text, maintenance and operating
instructions, runbooks, API and CLI docs, error messages, and prompts or tool
descriptions written for another agent. Do not apply to prose written in the user's own
voice, marketing text, or quoted source material — say so and stop rather than flattening
someone's writing.

Ask which mode applies if it is not obvious, because the rules differ:

- **Procedural** — steps the reader performs. Imperative, one instruction per sentence,
  20 words maximum.
- **Descriptive** — how a thing works. Simple present, 25 words maximum, limited passive.

## Procedure

1. Classify each block as procedural or descriptive. Split blocks that mix the two.
2. Rewrite sentence by sentence against the rule digest below, not paragraph by paragraph.
3. Check content words against `references/dictionary.md`. Grep, do not read the whole
   file: `grep -in '^| begin ' references/dictionary.md`. When a word is not listed,
   swap it for the nearest listed word — the reader gains more from a small vocabulary
   than from the exact word.
4. Keep technical names (`propeller`, `retaining bolt`, `overhead panel`) and technical
   verbs for shop processes (`to ream`, `to drill`). These are permitted outside the
   dictionary. Vocabulary control targets the connective tissue, not the parts list.
5. Re-read the finished text against the checklist below before you hand it over. Most
   violations are mechanical and survive a first pass unnoticed.
6. Report the rewrite plus the specific rule each change serves. Never silently change
   meaning: if a source sentence is ambiguous, ask which reading is correct rather than
   picking one.

## The rules that actually bite

Full paraphrase of all 9 sections in `references/rules.md`. These are the ones that
account for most violations:

**Words.** One word, one meaning, one part of speech. One name for one thing, every time
— never vary a term for style. No synonyms of an approved word: `start` is approved, so
`begin`, `commence`, and `initiate` are not. No idioms, slang, or jargon.

**Noun clusters.** Three words maximum. Break longer ones apart with prepositions:
`runway approach light indicator lamp` → `the indicator lamp for the runway approach
lights`.

**Verbs.** Only the infinitive, the imperative, the simple present, the simple past, the
simple future, and the past participle used as an adjective. No `-ing` forms except
inside a technical name (`landing gear`). No compound auxiliaries. Active voice always in
procedures; passive only in descriptive text when the actor is genuinely unknown.

**Sentences.** 20 words maximum in a procedure, 25 in a description. One instruction per
sentence. Never drop an article, a subject, or a verb to hit the count — ambiguity costs
more than words do.

**Paragraphs.** One topic each, six sentences maximum, topic sentence first. Use vertical
lists for anything with parts.

**Procedures.** Start with the command verb. Put a condition before its instruction:
`Before you remove the cover, disconnect the power.` Do not join two actions with `and`.

**Safety.** A warning covers injury; a caution covers damage to equipment. Both go before
the step they protect, never after. Start with the command or the condition, then give
the consequence in one short sentence.

**Ambiguity killers.** `must` for a requirement, `can` for a possibility — not `shall`,
`should`, `may`, or `might`. No `and/or`, no `/` as a conjunction, no parentheses around
information the reader needs.

## Final check

Walk the finished text once for each of these. They account for nearly every violation,
and they all survive a first pass unnoticed:

1. A sentence over 20 words (procedural) or 25 (descriptive).
2. An `-ing` word that is not part of a technical name (`landing gear`, `warning light`).
3. `be` plus a past participle inside a procedure — a passive hiding who must act.
4. A noun cluster over three words.
5. A step carrying two instructions.
6. A condition that arrives after its action.
7. A warning or caution that arrives after the step it protects.
8. A word in the swap table of `references/dictionary.md`, or a concept named two
   different ways in the same document.
9. `shall`, `should`, `may`, `and/or`, a semicolon, or a vague qualifier
   (`carefully`, `as necessary`, `appropriate`).

## Output format

When rewriting, give the STE text first, plainly, ready to paste. Then a short table of
the changes that were not obvious:

```markdown
| Original | STE | Rule |
|---|---|---|
| Prior to commencing the removal | Before you start the removal | approved word, condition first |
| The panel should be inspected | Examine the panel. | active imperative, approved word |
```

When auditing rather than rewriting, list findings worst first: quote the sentence, name
the rule, give the fix. Do not pad the list — a clean sentence needs no comment.

Close with one line naming the habit that produced the violations, so the next draft
avoids it. No praise, no summary of the document.

## Rules for this skill

- Prefer the listed word over the perfect word. Swapping vocabulary down is the method,
  not a compromise.
- The bundled dictionary is a working subset. For a contractual compliance claim, point
  the user at the official spec — free from https://www.asd-ste100.org (Issue 9, January
  2025).
- STE constrains form, not content. Never drop a caveat, a value, or a step to satisfy a
  word count. Split the sentence instead.
- Short does not mean terse. Keep the articles and the connecting words; the standard
  explicitly forbids telegraphic style.

## Additional resources

- **`references/rules.md`** — all 9 sections paraphrased, with worked before/after pairs
  for noun clusters, `-ing` forms, conditions, and safety text.
- **`references/dictionary.md`** — grep-first vocabulary: core approved words, the
  non-approved → approved swap table that resolves most rewrites, and the traps where an
  approved word carries a second, banned meaning.
