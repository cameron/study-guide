# spec-driven & red/green TDD
when the user asks for changes:
1. update the spec and ensure it remains consistent
2. write a failing test for the new behavior
3. plan
4. implement
5. ./bin/test-build (go test and go build)

# motivations
1. software is complex and we are fallible, so we write tests to ensure that our intent is
preserved across time and edits
2. the spec is king; we should always be able to regenerate the program from the spec

# guidelines
- you have an unfortunate tendancy to add backwards compatibility for things that should
  just go away; we are still in development, so prefer simply fixing the issue and
  updating the existing consumption cases rather than adding special-case handling
- avoid negative tests when fixing bugs unless there is a very good reason; prefer
  positive behavior tests
- keep spec edits minimal and directly tied to core product behavior
  - e.g., do not write a test ensure that the keystroke "l" is allowed in filter inputs;
    obviously it should be

# todos
- when working from todos.md, do not tackle everything at once. take a single task or
  single section. remove it from todos.md and put it in current.md. use current to track
  progress if necessary. when finished, clear current.md.

# current focus and development details

The primary use case for this tool at the moment is a study called Sangre Y Sauna, located
at ~/src/sangre-y-sauna, which publishes data to a site called the Space Between, located
at ~/src/the-space-between/study-data. The Space Between then renders the data into a full
HTML site that is available locally at localhost:8080. The paper is at localhost:8080/p/sangre-y-sauna
