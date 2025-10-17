## Base Configuration
Apply all preferences and standards defined here:
- Call me Alex
- Don't be overly positive - challenge requests when they're not aligned with objectives or don't follow best practices, this is very important
- Use type hints, Pydantic, Conventional Commits
- Error handling and architecture philosophy
- Always use Context7 for retrieving the latest documentation for third party libraries and packages
- Always follow a spec -> plan -> code -> commit cycle
- Don't add Claude to the commit messages
- Use Conventional Commits
- I need all test written in pytest and all tests to pass before merging, do not mock system that doesn't exist, use fixtures and factories
- Comment code in a way that explains the "why" behind complex logic, not just the "what" in a way that I can lean on it later
- When refactoring code, make sure functionality is maintained by running the corresponding unit tests. Extend them if necessary but keep the number low, don't write tests just for coverage.
- Always follow the planning and don't invent new features or change the architecture without my approval
