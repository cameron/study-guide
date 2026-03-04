# Aim & Motivation
Let's plan and build a CLI for defining a study and collecting data.

The specific motivation is the pain of applying structured metadata to photos captured during live
blood analysis using a microscope. Using the regular camera app leaves me with an annoying
problem of  post-hoc manual entry of metadata via the Photos app UI, which is cumbersome
and limited. Not only that, but my Photos library gets polluted with lots of blood photos
that I dont want to look at on a regular basis.

After designing a schema for the photo sample metadata (including study, protocol,
session, and sample), I realized I was very close to having a framework for automated
research paper generation.

# Implementation & Arch

let's keep it extremely simple:

- a CLI for defining and associating metadata sets with timestamps (now)
  - https://github.com/charmbracelet/bubbletea
  - https://github.com/charmbracelet/bubbles
  - rendering: https://github.com/charmbracelet/glow
- a flask web server with mobile-friendly form that exposes the CLI's subject
  creation feature

## persistence model (file system)

- study-guide/subject/<filename>.yaml (available globally to all studies/sessions)
  email or b) name with numeric suffix if necessary for uniqueness)
- <study>/
  - study.sg.md
  - protocol/<protocol>/
    - protocol.sg.md
    - session/<session>/
      - session.sg.md
      - sample/<sample>/
        - sample.sg.md
        - <img_xxx.jpg>,...

# Schema & Persistence

we'll store all the entity data on the file system in a folder hierarchy and <filename>.sg.md files.

sg <~ study-guide

we'll use markdown frontmatter for metadata like created_on, and markdown sections for
prose and more free-form data


## subject
location: `study-guide/data/subject/<filename>.sg.md`
filename: <email or name + first segment of uuid>.sg.md

- uuid
- type (enum: person)
- name*
- notes

### type: person (fields included on <subject>.sg.md)
- email
- phone
- age
- sex

## study
location `./study.sg.md`

- name
- PIs (via subjects that are people)
- status (enum: WIP, concluded)
- result status (enum: positive, negative, null, uncertain)
- hypotheses
- protocols (subfolder)
- created_on
- updated_on
- discussion
- special thanks to
- conclusion

## protocol(s)
location: `./protocol.sg.md` or if plural `./protocol/<short-name>/protocol.sg.md`
- name (optional if singular)
- description (first paragrpah of text in protocol.sg.md)
- steps
  - name
  - shortname (if other than transformed name, e.g., 'establish-a-baseline')
  - description
- sessions (subfolder `session/`)
- subject required fields (subject-requirements.yaml)

## session
location: `./protocol/<protocol>/session/<session-name>/`
session-name: <created_on>-<subject list>
subject-list: <surname[-surname,...]>
filename: session.sg.md

- subject(s)
- notes
- steps (subfolder `step/<step name>`)

## step instance
location: `./session/<session-name>/<step-name>/`
- time_started (derive from first artifact timestamp)
- free form markdown (aside from the frontmatter, no particular md is required)
- data (images, etc)


# CLI

`sg` is the executable name

## sg subject

### sg subject create

- must be created w/r/t a specific protocol
- if run from within a study directory, provide an autocomplete list of protocols (if more
  than one available), or select the only one
- collect required fields and prompt for optional

### sg subject search <name>

search subjects by name

### sg subject print <id>

print subject info by id

### sg subject ls

print list of subjects by name

### sg subject rm <id>

- rm

### web-based subject creation (human signup)

when i do analysis of another person's blood, i want them to have already created a
profile via a web link on their own phone and given some basic info
- name
- email address or phone number (for receiving blood photos)
- age, sex, free-form information

this should be done via a basic web server that writes to an sg.md file

the signup form should accept a query parameter with a study-protocol that can specify
subject requirements (e.g., age, gender, email, etc)


## sg init

create initial files/folders:
- study.sg.md
- protocol.sg.md
- subject-requirements.sg.md
- sessions/

## sg publish

generate a web page & pdf


## sg status

list of any files with missing sections (e.g., sessions with incomplete data, or study
sections like discussion/conclusion) necessary for publication


## sg session

- interactive:
  - subject selection (create session/<session-name>/session.sg.md)
  - start protocol (create step/<step>/step.sg.md with appropriate time_started)
  - advance step on user input (create next step)
  - finish


## sg ingest-photos

copies photos from the local Photos library into corresponding step folders by the photos'
creation time and the steps' time_started/time_finished (time_finished is inferred from
the subsequent step's time_started, except for the last step, which records an explicit time_finished)


# Sample Study

A sample study has been sketched out in files and folders under `sample-eg/`. Please
use it for reference and as a mirror for the above spec.


# Nice To Haves

- configurable session folder name
  - e.g., update to add h:m:s
