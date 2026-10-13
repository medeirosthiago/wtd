# wtd

An unofficial [worldtimebuddy](https://www.worldtimebuddy.com) for the terminal. It shows
your places side by side on a 24-hour strip and tells you what "9am MST" means
for you.

Built in one shot from a single prompt to Claude.

## Install

```sh
go install github.com/medeirosthiago/wtd@latest
```

## Usage

```sh
wtd                        # your places, now
wtd mst 09:00              # what 09:00 MST is for you and everyone else
wtd tomorrow 15:30 tokyo
wtd browse                 # interactive: ←/→ hours, / ask, a add, ? help

wtd add rio, salt lake city, taipei
wtd rm taipei
wtd home berlin
wtd clock 24
```

Places can be cities (`rio`, `valencia, spain`), countries (`brazil`, `DE`),
abbreviations (`pst`, `cest`, `pt`), offsets (`utc+5:30`) or IANA names
(`America/Denver`). The config lives in `~/.config/wtd/config.json`.

## License

MIT. City data from [GeoNames](https://www.geonames.org), filtered and
reformatted, under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
