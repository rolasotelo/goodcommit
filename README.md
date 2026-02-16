# Goodcommit

A CLI tool to help streamline writing good and useful commit messages. The spark that originated this idea was the realization that traditional commit messages often focus on what changed, but they rarely capture the reasoning behind the change, so I decided to create a tool that would help me (for me to) write better commit messages.

## Why Goodcommit?

Git excels at preserving what has changed in a codebase, but it needs a little help to capture the reasoning behind these changes. Goodcommit aims to bridge this gap by:

- Encouraging developers to think about the context and motivation behind their changes
- Providing structured templates that guide better commit message writing
- Ensuring consistency across team members and projects
- Making code history more valuable for future developers (including yourself)

It you allow me to sound grandiose, the goal is to transform commit messages from simple change logs into meaningful documentation of your development decisions.

## Overview

In essence, Goodcommit is a customizable commit message builder that ensures your commit messages follow best practices and are consistent across your projects. It is written in Go and uses [Charm](https://charm.land) packages for the CLI interface.

## Features

- **Interactive Interface**: Beautiful CLI interface built with [Charm](https://charm.land) packages
- **Customizable**: Allow users to define and use their own plugins in addition to the built-in ones

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
