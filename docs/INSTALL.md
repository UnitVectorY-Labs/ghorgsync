---
layout: default
title: Installation
nav_order: 2
permalink: /install
---

# Installation
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

## Installation Methods

There are several ways to install `ghorgsync`:

### Download Binary

Download pre-built binaries from the [GitHub Releases](https://github.com/UnitVectorY-Labs/ghorgsync/releases) page for the latest version.

[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/ghorgsync.svg)](https://github.com/UnitVectorY-Labs/ghorgsync/releases/latest) 

Choose the appropriate binary for your platform and add it to your PATH.

### Install Using Go

Install directly from the Go toolchain:

```bash
go install github.com/UnitVectorY-Labs/ghorgsync@latest
```

### Build from Source

Build the application from source code:

```bash
git clone https://github.com/UnitVectorY-Labs/ghorgsync.git
cd ghorgsync
go build -o ghorgsync
```