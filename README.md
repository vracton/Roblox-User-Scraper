# Roblox-User-Scraper

This is a tool that scrapes user data from the Roblox website. It's built with Go and uses [Rod](go-rod.github.io/) to handle web automation.

## Features
- Scrapes details about users (specifics below)
- Supports cookie loading to gather About Me data using a Roblox account
- Supports multithreading to spawn workers that work in parallel

## Constraints

Unfortunately, this scraper is rather slow. On my machine, it took an average of 3.31 seconds per user with 10 workers and 0.49 seconds per user with 100 workers (while using >70% CPU and 4GB of RAM). With over 8.8 billion Roblox users as of writing, it would be impossible to collect all their data, unless many computers worked together (perhaps for another project :)).


*NEW*
- With fixed multithreading, 100 users takes 10 workers 0.73 seconds per user.
- With better existence checking, 100 users takes 10 workers 0.56 seconds per user.
- It took 5 minutes to get data for 497 (there's an error which always pauses here...) users with 15 workers.

## Data Format

```json
[
    {
        "id": 1,
        "exists": true,
        "name": "@Roblox",
        "displayName": "Roblox",
        "about": "About Me...",
        "pfpURL": "<Base64 data URI>",
        "verified": true,
        "joinDate": "2/27/2006",
        "placeVisits": 11390237,
        "followersCount": 0,
        "followingCount": 0,
        "friendCount": 0,
        "friends": [1, 2, 3, 4, ...]
    },
    ...
]
```

## Usage

Installation
```golang
go get github.com/go-rod/rod
```
Run
```golang
go run main.go
```
