---
name: maintain-doc-tone
description: Use whenever you are asked to create, edit, modify, or review any documentation files, or whenever the user mentions writing documentation.
license: GNU GPL v3
metadata:
  category: workflow
  audience: general-coding-agent
  maturity: draft
  kind: reference
---

# Maintain Documentation Tone

Use this skill to ensure that all documentation in this repository maintains a consistent, friendly, and enthusiastic voice.

## Use this skill when
- The user asks to write, edit, or update a `.md` file in the documentation.
- You are adding a new page to the Astro site.
- The user asks you to review documentation for tone or style.

## Do not use this skill when
- Editing strict technical contracts (like OpenAPI specs or internal JSON schemas) where a conversational tone would cause parsing errors or confusion.
- The user explicitly asks for a formal, dry, or academic tone for a specific file (like an official legal disclaimer).

## First move
1. Read the exact tone requirements and examples located in [`references/tone-guidelines.md`](./references/tone-guidelines.md).
2. Scan the document you are about to edit to ensure your changes will blend seamlessly with the surrounding text.

## Workflow
1. Apply the tone guidelines (enthusiastic, accessible, emoji-friendly) to your draft.
2. Replace robotic or overly corporate phrasing ("This application executes the initialization sequence") with friendly, human phrasing ("Getting started is a breeze! Just run this command:").
3. Ensure formatting uses modern markdown features (bolding for emphasis, bullet points for readability).

## Validation
- **Check the emojis:** Does the section have at least one tasteful, relevant emoji?
- **Check the voice:** Read the text out loud (in your head). Does it sound like a friendly developer pairing with you, or does it sound like a corporate manual? If the latter, rewrite it.

## Examples
- "Update the Quick Start guide to include the new config flag." -> *Triggers this skill to ensure the new step is written enthusiastically.*
- "Write an ADR for switching to Astro." -> *Triggers this skill to ensure the ADR is clear but still approachable.*
