# Jamsync

This repository is a temporary place for Jamsync code until we're able to bootstrap on [jamsync.dev](https://jamsync.dev).

Happy syncin'!

## About
Jamsync is an open-source version control system that enables software engineers to develop and deploy faster. We're currently under development.

## Algorithm
The idea behind Jamsync is the same as rsync. In fact, Jamsync uses jbreiding/rsync-go for now under the hood. If you havent read the rsync algorithm paper, it's highly recommended -- it's very approachable for anyone with a computer science background and has some great information.

## Rsync
Essentially, rsync allows a file on two separate computers, a sender and receiver, to be synced efficiently over a network. It does this by chopping a file into blocks and reusing existing blocks on the receiver to regenerate a file. A rolling hash is used to scan over the file on the sender and operations are sent to the receiver to either reuse a block (if the rolling hash matches) or to use new data.

## Changes and Conflicts
A chain of changes, formed by the process above, can be used to regerate every file in a project. Branches off of the main chain can be used to prevent conflicts from occuring when editing the same part of a file; however, whenever a branch is merged in, every other branch is automatically rebased on top. This means that every branch will always be up-to-date. If conflicts occur during the rebase, a branch will be marked as "stale" and will need manual merging.

## Limitations
The goal is to be able to handle over 10M files and over 1TB-sized files in a single repository. We're not there yet in the current implementation (probably ~10K files with 1GB-sized files) but should be there in the next couple months.

## Developing

### Setup

Note that this is for setting up development or compiling Jamsync yourself. If you want binaries and installation instructions go to [jamsync.dev/download](https://jamsync.dev/download)

1. Install Go, Protoc, Make
2. Setup env with Auth0 variables
3. Run desired `make` target

If running on server, make sure to setup the systemd services using the files in `systemd/`. Optionally, you can setup ssh.prod.jamsync.dev to resolve to the server address in your hosts to use some additional `make` targets.

### Architecture

There are three services that currently compose Jamsync -- web, server, client.

1. Web - runs the REST api and website
2. Server - runs the backend server for storing and retrieving changes
3. Client - client-side CLI tool to connect to the server

The client and web services connect through gRPC to the backend server and we interact online through the web REST API. More documentation will be added in the future to detail how and where changes are stored.

## Contributing
We're currently not open to contributions, but feel free to contact us if you would like us to change anything! 

## Contact

Email Zach Geier at [zach@jamsync.dev](mailto:zach@jamsync.dev).