# ASD-STE100 writing rules — working paraphrase

The standard is Part 1 (53 rules in 9 sections) plus Part 2 (the dictionary). What
follows paraphrases the rule *categories* for working use. It is not the standard's text.
Rule numbers appear only where they are stable and widely cited; elsewhere the section
name is enough. Current edition: Issue 9, January 2025, free from
https://www.asd-ste100.org.

---

## Section 1 — Words

**Use only approved words.** A word is available if the dictionary approves it, or if it
qualifies as a technical name (rule 1.5) or a technical verb (rule 1.12). Nothing else.

**One word, one meaning, one part of speech.** The approved entry fixes both. `close`
(verb) means to shut. It does not mean near — use `near`. `follow` means to come after in
a sequence; it does not mean obey — use `obey`. If a word you want is approved only as a
noun, you cannot press it into service as a verb.

**One term per concept, forever.** Pick the term and repeat it. Elegant variation is a
defect: `bolt`, then `fastener`, then `retaining device` reads as three objects to someone
who does not know they are one.

**No synonyms of an approved word.** `start` is approved, so `begin`, `commence`,
`initiate`, and `kick off` are all unavailable — the point is not that they are long, it
is that the reader has only one word to learn.

**Technical names (1.5).** Nouns for the things being documented are permitted with no
dictionary entry: parts, materials, tools, locations, standards, circuits, substances.
`propeller`, `grease`, `discoloration`, `aural warning system`, `overhead panel`. Use the
official name from the parts list; do not coin a second one.

**Technical verbs (1.12).** Verbs for shop and manufacturing processes are permitted when
no approved verb carries the meaning: `to ream`, `to drill`, `to countersink`, `to
crimp`. This is a narrow door, not a general escape hatch — if `remove` will do, use
`remove`.

**Banned classes.** No slang, no idiom, no colloquialism, no metaphor, no humour, no
rhetorical question. `bleed the line` is fine as a technical verb; `it is a piece of
cake` is not English to half your readers.

**Abbreviations.** Use only those on the project's approved list. Spell out on first use.
Do not invent one to save a word.

**Spelling.** One convention throughout the document set. Do not mix `color` and
`colour`.

---

## Section 2 — Noun clusters

**Three words maximum.** A noun cluster is a run of nouns acting as one modifier. Past
three, the reader cannot tell what modifies what.

| Not this | This |
|---|---|
| runway approach light indicator lamp | the indicator lamp for the runway approach lights |
| fuel pump pressure relief valve | the pressure relief valve of the fuel pump |
| authentication token refresh failure count | the number of failures when the system refreshes the authentication token |

Break the cluster with prepositions (`of`, `for`, `in`) or with a relative clause. If the
cluster is an official technical name, keep it whole and define it once at first use.

---

## Section 3 — Verbs

**Permitted forms only:**

- infinitive — `to remove`
- imperative — `Remove the cover.`
- simple present — `The valve controls the flow.`
- simple past — `The test failed.`
- simple future — `The lamp will come on.`
- past participle as an adjective — `the damaged bolt`, `the approved procedure`

**No `-ing` forms.** No gerunds, no present participles, no progressive tenses. The
exception is an `-ing` word inside a technical name: `landing gear`, `bearing`, `warning
light`, `housing`.

| Not this | This |
|---|---|
| When installing the bracket, hold the cover. | When you install the bracket, hold the cover. |
| Loosening the bolt releases the tension. | If you loosen the bolt, the tension is released. |
| The pump is supplying pressure to the line. | The pump supplies pressure to the line. |

**No complex verb constructions.** Nothing built from stacked auxiliaries: `would have
been checked`, `is going to be replaced`, `might have failed`.

**Active voice.** Mandatory in procedures. In descriptive text, use the passive only when
the actor is genuinely unknown or irrelevant, and never to hide who must act.

| Not this | This |
|---|---|
| The filter must be replaced by the technician. | Replace the filter. |
| It is recommended that the unit be tested. | Do a test of the unit. |

**Modals.** `must` for a requirement. `can` for a possibility or a permission. Avoid
`shall`, `should`, `may`, `might`, `ought to` — each carries two readings.

---

## Section 4 — Sentences

**Length.** 20 words maximum in procedural text. 25 in descriptive text. The cap is a
symptom check: a sentence over the cap usually contains two sentences.

**One instruction per sentence.** Two actions get two sentences, even when they are
adjacent. Only genuinely simultaneous actions share a sentence, and then say so: `Hold
the lever and turn the switch at the same time.`

**Do not drop words to fit.** Keep the articles, the subject, the verb, the `that`.
Telegraphic style is a defect, not a virtue: `Remove valve, check seat` is ambiguous;
`Remove the valve. Examine the seat.` is not.

**Show the relation.** Use a connecting word where a logical relation exists (`if`,
`because`, `then`, `but`). Do not leave the reader to infer causality from adjacency.

**No stacked clauses.** One main clause plus at most one subordinate clause. Convert the
rest into separate sentences or into a vertical list.

---

## Section 5 — Procedural writing

**Start with the command verb.** `Remove the panel.` — not `The panel is to be removed`
or `The technician removes the panel`.

**Condition first, action second.** The reader must know whether the step applies before
they read what it does.

| Not this | This |
|---|---|
| Disconnect the power before you remove the cover. | Before you remove the cover, disconnect the power. |
| Open the valve if the pressure is more than 5 bar. | If the pressure is more than 5 bar, open the valve. |

**Do not join actions with `and` or `or`.** `Open the valve and drain the tank` is one
step with two failure points. Number them separately.

**Vertical lists.** Use them for parts, conditions, tools, and any sequence of more than
two items. Number the list when the order matters; bullet it when it does not.

**Be specific.** `Tighten the bolt to 25 Nm.` — not `Tighten the bolt correctly.` Vague
qualifiers (`carefully`, `as necessary`, `appropriate`, `sufficient`) are instructions the
reader cannot follow.

---

## Section 6 — Descriptive writing

- Simple present for how a system works and what it does.
- 25 words maximum per sentence, six sentences maximum per paragraph.
- One topic per paragraph. Put the topic sentence first.
- Vary sentence length and construction inside the paragraph so the text stays readable —
  descriptive prose that is all 8-word sentences reads as a list.
- The limited passive allowance lives here and only here.

---

## Section 7 — Safety instructions

**Warning = injury or death. Caution = damage to equipment or data.** Do not mix them and
do not soften a warning into a caution.

**Position.** Before the step it protects. A warning that appears after the step has
already failed.

**Shape.** Start with the command or the condition, then state the consequence in one
short sentence.

```
WARNING: DO NOT TOUCH THE TERMINALS. HIGH VOLTAGE CAN KILL YOU.

CAUTION: DO NOT USE A SOLVENT ON THE SEAL. A SOLVENT CAUSES DAMAGE TO THE RUBBER.
```

Keep the same imperative rules as any other procedure: active, specific, one instruction
per sentence. State the hazard, not a feeling about the hazard — never `be careful`.

---

## Section 8 — Punctuation and word count

- No `/` as a conjunction, and no `and/or`. Write both options, or write `or`.
- No parentheses around information the reader needs. Parenthetical text is skipped text.
- Hyphenate a compound modifier when the dictionary or the project list does.
- No semicolons joining two independent clauses — that is two sentences.
- Numbers, units, and part references follow the project standard, used identically
  everywhere in the document set.

---

## Section 9 — Document and text structure

- Consistent, informative headings; the reader navigates by them.
- Tables and vertical lists in place of dense paragraphs whenever the content has parts.
- Sequential steps numbered, so a reader can be told to go to step 4.
- Cross-references point to a specific place, never to `the above` or `the following`.
- The same information appears in one place only. Duplicated text drifts apart.

---

## Fast audit checklist

1. Any sentence over 20 words (procedural) or 25 (descriptive)?
2. Any `-ing` word that is not part of a technical name?
3. Any form of `be` plus a past participle in a procedure?
4. Any noun cluster over three words?
5. Any step with two instructions in it?
6. Any condition that arrives after its action?
7. Any warning or caution that arrives after its step?
8. Any word not in the dictionary and not a technical name?
9. Any concept named two different ways in the same document?
10. Any `shall`, `should`, `may`, `and/or`, or vague qualifier?
