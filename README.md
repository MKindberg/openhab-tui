A tui for openhab built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [wish](https://github.com/charmbracelet/wish) accesible either as a program or over ssh. It will create a layout based on a sitemap of your choice and currently support Switch, Slider and Frame.

# Usage

Just running the binary will try to find an openhab server on localhost and try to open a tui for a sitemap called `default`. This can be configured with command line options

| Option  | Default   | Description                                 |
| ---     | ---       | ---                                         |
| ip      | localhost | IP if the openhab server.                   |
| sitemap | default   | Name of the sitemap to use.                 |
| server  | -         | Run as a server accesible via ssh instead.  |
| host    | localhost | Hostname to run server on.                  |
| port    | 23234     | Port to run server on.                      |

So if we for example run `openhab_tui -server -sitemap tui` we can use `ssh localhost -p 23234` to access a tui version of the sitemap called tui.

# Installation

Clone the repo, install go and run `go install .`. To cross-compile for a raspberry pi `GOOS=linux GOARCH=arm GOARM=5 go build .` can be used.
