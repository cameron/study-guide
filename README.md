# Study Guide

_A CLI tool for collecting and organizing data from sources that are blind to your
research protocol (like your photo library)._

## Motivation: Multisubject Live Blood Analysis Studies
_But the solution generalizes._

I've been doing some live blood analysis, and when juggling multiple subjects (people) in
a multi-step protocol, it quickly becomes a mystery whose photos are whose&mdash;unless you are
annotating or organizing each photo as you take it (via the phone's photo app??), which is a drag.

Ideally, the researcher should only have to:
1. ahead of time, define a protocol (series of steps)
2. during the study, create a sessions (a protocol and a subject, e.g., a person)
3. advance each session through its steps (CLI will record timestamps)
4. indicate which session is currently being captured by the phone or external data source
   (CLI records more timestamps)
5. profit

Having recorded the timestamps of each session's steps and focus windows, the program can
then automatically import photos from the camera roll into the appropriate session-step folder.

No more manual metadata annotation.

While the only current data source supported is OS X's Photos.app, it would be
straightforward to add other sources.

# `sg`: A CLI for Running a Simple Study

The `sg` CLI solves the above problem&mdash;and then goes a bit further, because once you've given your
study a name, defined a protocol, and collected some data, you're a couple of paragraphs
of prose (abstract, discussion, conclusion) away from having everything you need to
publish a paper.

## Interactive DWIM by Default

The naked command is designed to be interactively feature-complete; that is, all you ever have to
do is get into your (potentially empty) study directory, run `sg`, and it will DWIM ("Do
what [you] mean"): initialize the study, protocol, and session files and folders, help you
outline the protocol, create, manage, and focus sessions, publish a PDF and HTML page&mdash;all
driven by the state of the file system.

## Non-interactive Mode

`sg` also aims to provide all features non-interatively, that is, as one-off subcommands
of the CLI.

Run `sg -h` to see a list of available commands.

## Persistence: Markdown Files

`sg` uses no database; aiming to be editable by non-technical users with basic text
editors, all state is stored in markdown and yaml files:

```text
my-simple-study/
  study.sg.md
  protocol.sg.md
  subject-requirements.yaml
  session/
    <session-slug>/
      session.sg.md
      step/
        <step-slug>/
          step.sg.md
          asset/
            <ingested-files>
```

The study directory stores study/protocol/session data. Subjects themselves live in a
global store outside the study tree:

```text
~/.study-guide/
  subject/
    <subject>.sg.md
```

# Status: Alpha

While you can probably clone, build, and use the tool if you have a working Go
installation, I haven't yet put much effort into making the process smooth, or documenting
much.

Feel free to open an issue if you'd like to use it and are having trouble.

## Development Notes

This project itself has been an experiment&mdash;in frameworkless, spec-driven, agentic development.

See `agents.md` for some basic notes that guide the agent, and `spec.md` for the source of
product truth.

# Acknowledgements

Thanks to Charm for the lovely TUI toolkit.

- https://charm.land/bubbletea/v2
- https://charm.land/bubbles/v2
- https://github.com/charmbracelet/glow
