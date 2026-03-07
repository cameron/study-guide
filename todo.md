- [x] standardize styles
  - [x] selected row/option style
    - [x] use background instead of text color
    - [x] remove legacy text-only selection indicators from sessions browse/create views
  - [x] CTA (buttons/actions)
    - [x] transparent green for unselected buttons/actions
    - [x] bright green for selected

- [x] test for subject requirements being read and absorbed
  - [x] add a custom field to `subject-requirements.yaml` and ensure it is prompted for in
    create-subject flow

- [x] selected / inactive row currently shows `activate` in active column when cursor is on
  `next step`
  - [x] should be empty

- [x] big change to sessions table
  - [x] change language from `active/activate` to `focused/focus`
  - [x] focused row
    - [x] always on the top
    - [x] light blue background
  - [x] layout update
    - [x] headers
    - [x] focused session summary line
    - [x] empty spacer line
    - [x] `filter:`
    - [x] open sessions table

- [x] add "step backwards" command
  - [x] `sg session reverse`
  - [x] erases `time_started` on the current step (leaves folder intact)
  - [x] `ctrl+b` in the sessions switchboard
  - [x] test persistence by stepping backwards and then forwards, asserting `time_started`
    is rewritten
