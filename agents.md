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
  just go away; we are still in development, so prefer simply fixing the issue and updating the existing
  consumption cases rather than adding special-case handling
