<p align="center">
  <img width="300" height="300" src="assets/driftive.png">
</p>

# Driftive

Driftive is a simple tool for detecting drift in Terragrunt/Terraform projects.
Currently, it supports only Terragrunt projects.

## Features
* Concurrently analyze multiple projects
* Notifies about drifts in Slack

## Installation

### CLI

Homebrew
```bash
$ brew install driftive/tap/driftive
```

## Usage

CLI usage:
```bash 
$ driftive --help
```

Docker usage:
```bash
docker pull driftive/driftive:0.2.0
docker run driftive/driftive:0.2.0 --help
```

### Output example

A message will be sent in the Slack channel if any state drift is detected.
Example:

````
:bangbang: State Drift detected in Terragrunt projects
:gear: Drifts 2/14
:clock1: Analysis duration 1m.30s
:point_down: Projects with state drifts
```
my/project1
my/project2
```
````


