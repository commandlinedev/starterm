<p align="center">
  <a href="https://www.starterm.dev">
	<picture>
		<source media="(prefers-color-scheme: dark)" srcset="./assets/star-dark.png">
		<source media="(prefers-color-scheme: light)" srcset="./assets/star-light.png">
		<img alt="Star Terminal Logo" src="./assets/star-light.png" width="240">
	</picture>
  </a>
  <br/>
</p>

# Star Terminal

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fcommandlinedev%2Fstarterm.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fcommandlinedev%2Fstarterm?ref=badge_shield)

Star is an open-source terminal that combines traditional terminal features with graphical capabilities like file previews, web browsing, and AI assistance. It runs on MacOS, Linux, and Windows.

Modern development involves constantly switching between terminals and browsers - checking documentation, previewing files, monitoring systems, and using AI tools. Star brings these graphical tools directly into the terminal, letting you control them from the command line. This means you can stay in your terminal workflow while still having access to the visual interfaces you need.

![StarTerm Screenshot](./assets/star-screenshot.webp)

## Key Features

- Flexible drag & drop interface to organize terminal blocks, editors, web browsers, and AI assistants
- Built-in editor for seamlessly editing remote files with syntax highlighting and modern editor features
- Rich file preview system for remote files (markdown, images, video, PDFs, CSVs, directories)
- Integrated AI chat with support for multiple models (OpenAI, Claude, Azure, Perplexity, Ollama)
- Command Blocks for isolating and monitoring individual commands with auto-close options
- One-click remote connections with full terminal and file system access
- Rich customization including tab themes, terminal styles, and background images
- Powerful `wsh` command system for managing your workspace from the CLI and sharing data between terminal sessions

## Installation

Star Terminal works on macOS, Linux, and Windows.

Platform-specific installation instructions can be found [here](https://docs.starterm.dev/gettingstarted).

You can also install Star Terminal directly from: [www.starterm.dev/download](https://www.starterm.dev/download).

### Minimum requirements

Star Terminal runs on the following platforms:

- macOS 11 or later (arm64, x64)
- Windows 10 1809 or later (x64)
- Linux based on glibc-2.28 or later (Debian 10, RHEL 8, Ubuntu 20.04, etc.) (arm64, x64)

The WSH helper runs on the following platforms:

- macOS 11 or later (arm64, x64)
- Windows 10 or later (arm64, x64)
- Linux Kernel 2.6.32 or later (x64), Linux Kernel 3.1 or later (arm64)

## Roadmap

Star is constantly improving! Our roadmap will be continuously updated with our goals for each release. You can find it [here](./ROADMAP.md).

Want to provide input to our future releases? Connect with us on [Discord](https://discord.gg/XfvZ334gwU) or open a [Feature Request](https://github.com/commandlinedev/starterm/issues/new/choose)!

## Links

- Homepage &mdash; https://www.starterm.dev
- Download Page &mdash; https://www.starterm.dev/download
- Documentation &mdash; https://docs.starterm.dev
- Legacy Documentation &mdash; https://legacydocs.starterm.dev
- Blog &mdash; https://blog.starterm.dev
- X &mdash; https://x.com/commandlinedev
- Discord Community &mdash; https://discord.gg/XfvZ334gwU

## Building from Source

See [Building Star Terminal](BUILD.md).

## Contributing

Star uses GitHub Issues for issue tracking.

Find more information in our [Contributions Guide](CONTRIBUTING.md), which includes:

- [Ways to contribute](CONTRIBUTING.md#contributing-to-star-terminal)
- [Contribution guidelines](CONTRIBUTING.md#before-you-start)
- [Storybook](https://docs.starterm.dev/storybook)

## License

Star Terminal is licensed under the Apache-2.0 License. For more information on our dependencies, see [here](./ACKNOWLEDGEMENTS.md).
