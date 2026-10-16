---
title: vlx — the Varlink explorer
description: vlx is a beautiful terminal UI and CLI for browsing and calling Varlink services such as systemd's io.systemd.* APIs. Read-only by default.
hide:
  - navigation
  - toc
---

# vlx { .vlx-hidden }

<div class="vlx-hero" markdown>
<img src="assets/banner.png" alt="vlx — the Varlink explorer for your terminal">
<div class="vlx-hero__body" markdown>
<p class="vlx-hero__lead">
<strong>vlx</strong> finds every Varlink socket on your machine and lets you
browse its interfaces, read the documentation that ships with them, and call
methods with an editor that already knows the schema — <strong>without ever
changing your system unless you ask it to.</strong>
</p>
<p class="vlx-hero__actions">
<a class="md-button md-button--primary" href="getting-started/quickstart/">QuickStart</a>
<a class="md-button" href="https://github.com/rossedman/vlx">GitHub</a>
</p>
<div class="vlx-hero__install">go install github.com/rossedman/vlx/cmd/vlx@latest && vlx demo</div>
</div>
</div>

<div class="vlx-cards" markdown>
<div class="vlx-card" markdown>
:material-magnify-scan:
### Discover
Run `vlx`. It lists the Varlink sockets under `/run/systemd`, `/run/varlink/registry` and friends — no flags, no config.
</div>
<div class="vlx-card" markdown>
:material-file-tree:
### Browse
Interfaces appear as you move. ++ctrl+k++ searches every method, type and error on the machine at once.
</div>
<div class="vlx-card" markdown>
:material-code-json:
### Call
A form built from the schema — pickers for enums, inline docs, checks as you type. Replies stream in live.
</div>
<div class="vlx-card" markdown>
:material-shield-check:
### Stay safe
Read-only by default. Writes need `--allow-writes` *and* a confirmation; dangerous methods need their name typed back.
</div>
<div class="vlx-card" markdown>
:material-console:
### Script it
Everything the TUI does has a command: `vlx call … -o json`, stable exit codes, NDJSON streams.
</div>
<div class="vlx-card" markdown>
:material-school:
### Learn
`vlx demo` serves a sample service anywhere — macOS included — and every screen shows the equivalent `varlinkctl` command.
</div>
</div>

![The vlx explorer browsing the demo service](assets/screens/browse.png){ .vlx-shot }

## How it works

![vlx architecture: the TUI and CLI call through a safety gate and sdvarlink to Unix sockets](assets/architecture.png){ .vlx-diagram }

vlx is built on [sdvarlink](https://github.com/rossedman/sdvarlink), a pure-Go
Varlink library with an IDL parser, and on Charm's
[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss), with
[Cobra](https://github.com/spf13/cobra) and
[Viper](https://github.com/spf13/viper) for the command line.

[Get started in two minutes :material-arrow-right:](getting-started/quickstart.md){ .md-button .md-button--primary }
