- standardize styles
  - selected row/option style
    - use background instead of text color
    - some views still use pink text as a selection indicator
    - possible w/o breaking the table cursor? there was a lot of thrashing around this
      yesterday, so plan this very carefully
  - CTA (buttons/actions)
    - transparent green for unselected buttons/actions
    - bright green for selected

- test for subject requireemnts being read and absorbed
  - add a custom field to subject-requirements.sg.md and ensure it is prompted for in the
    create subject row

- selected / inactive row currently shows "activate" in active column's cell when cursor is on "next step"
  - should be empty!

- big change to sessions table
  - change language from "active/activate" to "focused/focus"
  - focused row
    - always on the top
    - light blue background
  - layout update
    - HEADERS
    - (active session)
    - (empty line)
    - filter:
    - (open sessions)

- add "step backwards" command
  - sg session reverse
  - erases the time_started on the current step (leaves folder intact)
  - ctrl-b in the sessions switchboard
  - make sure to test the persistence by using step backwards and then stepping forwards,
    asserting time_started is correct
