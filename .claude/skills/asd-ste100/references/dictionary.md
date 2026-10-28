# STE vocabulary — working subset

**What this is.** A grep-first working subset of the ASD-STE100 controlled vocabulary:
the swaps that resolve most rewrites, plus the core approved words they resolve to. It is
a paraphrase for drafting, not the standard. The official Part 2 dictionary — about 900
approved words and about 1,200 non-approved words, each with an approved meaning, one
part of speech, and STE / non-STE examples — is owned by ASD and is free to download from
https://www.asd-ste100.org. Use the official file for any compliance claim.

**How to use it.** Grep, do not read:

```sh
grep -in '^| begin '  references/dictionary.md   # is this word banned, and what replaces it?
grep -in '| start |'  references/dictionary.md   # what maps to this approved word?
```

**When a word is not listed,** take the nearest listed word instead. Swapping down is
always safe and is the point of the exercise — a reader who knows 900 words is served by
the 900, not by the one that was slightly more precise.

**Always allowed outside the dictionary:** technical names (parts, materials, tools,
locations, standards — `propeller`, `grease`, `overhead panel`) and technical verbs for
shop processes (`to ream`, `to drill`, `to crimp`). Also project-approved abbreviations,
numbers, and units.

---

## Non-approved → approved

Sorted alphabetically by the word to remove. `pos` is the part of speech in the offending
use.

| word | pos | use instead | note |
|---|---|---|---|
| accomplish | v | do | |
| acquire | v | get | |
| activate | v | start, energize | pick by what actually happens |
| additional | adj | more, other | |
| adjacent | adj | near, next to | |
| advise | v | tell | |
| aid | v | help | |
| allocate | v | give, assign to | |
| alternatively | adv | or | start a new sentence |
| ameliorate | v | make better | |
| approximately | adv | about | |
| ascertain | v | find out, make sure | |
| assist | v | help | |
| attempt | v | try | |
| avoid | v | do not | say the prohibited action |
| begin | v | start | |
| cease | v | stop | |
| commence | v | start | |
| comply with | v | obey | |
| component | n | part | keep `component` only as a technical name |
| comprise | v | have, contain | |
| concerning | prep | about | |
| consequently | adv | thus, so | prefer a new sentence with `Thus,` |
| considerable | adj | large, much | give the number if you have one |
| deactivate | v | stop, de-energize | |
| demonstrate | v | show | |
| depress | v | push | `depress` also means make sad |
| desire | v | want | |
| determine | v | find out, calculate | |
| discontinue | v | stop | |
| due to the fact that | phr | because | |
| effect | v | do, make | `effect` as a verb is banned; the noun is different |
| eliminate | v | remove | |
| employ | v | use | |
| enable | v | let, make possible | |
| encounter | v | find, get | |
| endeavour | v | try | |
| ensure | v | make sure | |
| entire | adj | all of, complete | |
| equivalent | adj | equal, the same as | |
| erroneous | adj | wrong, incorrect | |
| establish | v | make, set | |
| eventually | adv | later, at the end | |
| execute | v | do | |
| exhibit | v | show | |
| expedite | v | make faster | |
| facilitate | v | help, make easier | |
| following (= after) | prep | after | `following` is legal only as a verb form of `follow` |
| formerly | adv | before | |
| frequently | adv | often | |
| furthermore | adv | also | |
| generate | v | make, produce | |
| hazardous | adj | dangerous | |
| hence | adv | thus | |
| however | adv | but | start a new sentence |
| identify | v | find, show, mark | pick the real action |
| illustrate | v | show | |
| immediately | adv | now, at once | |
| implement | v | do, install, make | |
| in accordance with | phr | as in, per the procedure | name the document |
| in conjunction with | phr | with | |
| in order to | phr | to | |
| in the event that | phr | if | |
| in the vicinity of | phr | near | |
| indicate | v | show | |
| initiate | v | start | |
| inquire | v | ask | |
| inspect | v | examine, check | `examine` is the workhorse |
| insufficient | adj | not enough | |
| intend | v | plan, want | |
| interrogate | v | ask, read | |
| liaise | v | speak to, tell | |
| locate | v | find, put | two different actions — pick one |
| maintain | v | keep, do maintenance on | |
| majority of | phr | most of | |
| malfunction | n | failure, fault | |
| manufacture | v | make, build | |
| may | modal | can | `may` mixes permission and possibility |
| methodology | n | method | |
| minimize | v | make smaller, decrease | |
| modify | v | change | |
| monitor | v | look at, watch, examine | |
| necessitate | v | need, make necessary | |
| notify | v | tell | |
| numerous | adj | many | |
| observe | v | look at, examine, obey | pick the real action |
| obtain | v | get | |
| occur | v | happen | |
| operate (= work) | v | operate is fine for equipment | do not use it to mean `work on` |
| optimum | adj | best | |
| perform | v | do | `Do the test.` not `Perform a test.` |
| permit | v | let | |
| pertaining to | phr | about, of | |
| portion | n | part | |
| possess | v | have | |
| previously | adv | before | |
| prior to | phr | before | |
| procure | v | get, buy | |
| prohibited | adj | not permitted, do not | prefer the direct command |
| provide | v | give, supply | |
| purchase | v | buy | |
| reduce | v | decrease, make smaller | |
| regarding | prep | about | |
| relocate | v | move | |
| remainder | n | the other parts | |
| render | v | make | |
| request | v | ask for | |
| require | v | need | |
| reside | v | is in, is on | |
| retain | v | keep | |
| reveal | v | show | |
| rotate | v | turn | |
| shall | modal | must | `shall` is not a requirement to every reader |
| should | modal | must, or delete | decide whether it is a rule or not |
| significant | adj | large, important | give the number if you have one |
| situated | adj | is in, is at | |
| subsequent to | phr | after | |
| subsequently | adv | then, after that | |
| sufficient | adj | enough | |
| terminate | v | stop, end | |
| therefore | adv | thus | |
| thus far | phr | until now | |
| transmit | v | send | |
| ultimately | adv | at the end | |
| utilize | v | use | |
| various | adj | different, some | |
| verify | v | make sure, examine | |
| whilst | conj | while | |
| and/or | phr | and, or | write both options if you mean both |
| etc. | abbr | name the items | an incomplete list is an ambiguous list |
| e.g. | abbr | for example | |
| i.e. | abbr | that is | |
| n/a | abbr | not applicable | |

### Vague qualifiers — delete or quantify

`adequately`, `appropriate`, `as necessary`, `as required`, `carefully`, `correctly`,
`easily`, `frequently`, `if possible`, `periodically`, `properly`, `regularly`,
`suitable`, `sufficiently`, `when necessary`.

Each hides a number the reader needs. `Tighten the bolt to 25 Nm.` — not `Tighten the
bolt correctly.`

### Idioms and figurative use — never

`back up` (unless the technical term), `bring up`, `carry out`, `check out`, `come up
with`, `cut down on`, `deal with`, `figure out`, `hold off`, `look into`, `put in place`,
`run into`, `set up` (prefer `install`), `take care of`, `turn out`.

Phrasal verbs are the single worst offender for non-native readers: the parts do not
predict the meaning.

---

## Traps — approved word, wrong meaning

These words are approved, but only in one sense. Using the other sense is a violation
that no spell-checker catches.

| word | approved meaning | not this meaning | say instead |
|---|---|---|---|
| close | to shut | near | near, next to |
| follow | to come after in sequence | to obey | obey |
| free | not held, able to move | at no cost | no cost |
| just | exactly, only | recently | a moment ago |
| left | the direction | remaining | the other, remaining parts |
| like | to be similar to | to want, to enjoy | want |
| may | — | permission or possibility | can, must |
| open | to make open | not closed off, available | available |
| operate | to make equipment work | to work on | do work on |
| party | a person or group in an agreement | a social event | — |
| present | to show | now, current | now, at this time |
| right | the direction | correct | correct |
| second | the unit of time | next in a list | next, step 2 |
| since | — | because | because |
| well | in a good way | a hole in the ground | — |
| while | at the same time as | although | but, although → split the sentence |

---

## Core approved words

A subset of the approved vocabulary, grouped by job. These are safe defaults — when a
rewrite needs a word, take it from here first.

**Actions.** adjust, apply, attach, be, bend, break, calculate, change, clean, close,
connect, continue, cool, correct, cut, decrease, disconnect, do, drain, dry, energize,
examine, extend, fill, find, flush, get, give, go, heat, help, hold, increase, install,
keep, know, let, lift, listen, look, loosen, lubricate, make, measure, mix, move, obey,
open, operate, prevent, pull, push, put, read, release, remove, repair, replace, retract,
see, send, set, show, shut, speak, start, stop, supply, tell, test, tighten, touch, try,
turn, use, wait, want, wash, watch, wear, weigh, work, write.

**Things.** air, area, bottom, cause, centre/center, condition, damage, danger, data,
day, direction, distance, edge, end, example, failure, fault, fire, flow, force, front,
gas, ground, group, heat, height, hole, hour, item, layer, length, level, light, line,
liquid, list, load, material, method, minute, moisture, name, note, number, oil, order,
part, pressure, procedure, process, quantity, rate, reason, result, side, sign, size,
speed, step, surface, system, table, temperature, test, time, tool, top, type, unit,
value, volume, water, weight, width, work.

**Descriptions.** able, bad, clean, clear, cold, complete, correct, dangerous, dark,
dead, deep, different, difficult, dirty, dry, easy, electrical, empty, equal, external,
first, flat, full, good, hard, heavy, high, hot, internal, large, last, light, long,
loose, low, magnetic, many, more, most, narrow, necessary, new, next, normal, old, only,
open, other, ready, same, second, several, sharp, short, small, smooth, soft, straight,
strong, thick, thin, tight, usual, weak, wet, wide, wrong.

**Glue.** a, about, above, across, after, again, against, all, along, also, always, an,
and, any, around, as, at, away, because, before, behind, below, between, both, but, by,
can, do not, down, during, each, enough, every, for, from, here, how, if, in, into, less,
much, must, near, no, not, now, of, off, on, onto, or, out, over, per, so, some, than,
that, the, then, there, these, this, those, through, thus, to, together, under, until,
up, when, where, which, while, with, without, you.

**Pronouns.** Use `you` for the reader in procedures. Do not use `we`, `I`, `he`, `she`,
or `they` for people. Do not use `it` or `this` where the referent is more than one
sentence back — repeat the noun.
