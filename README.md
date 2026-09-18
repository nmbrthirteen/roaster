# Roaster

A conference kiosk that reads a developer's public GitHub activity and prints
them a roast on thermal paper. Built for the Upgaming stand: a visitor types
their handle, the audit runs while they watch, and an 80mm receipt comes out
with a QR code to the digital version and the open roles.

The whole thing is one Go binary with no runtime to install. The front end is
plain HTML, CSS and JavaScript with no build step.

## How the printing works

A receipt is a document of blocks, flattened to fixed-width lines. Two encoders
consume the same lines: one emits ESC/POS bytes for the printer, the other
emits HTML for the designer page. They cannot drift apart, so what you see in
the browser is what the head fires.

The printable width is 42 columns in Font A, which every 80mm printer supports.
Two columns are held at each edge as a margin, leaving 38 for content.

## Running it

```sh
go run ./cmd/roaster
```

Then open <http://localhost:3000>. That is the receipt designer: it renders the
document, shows the paper length and job size, and prints.

Settings live in `roaster.json` beside the binary. Copy `roaster.example.json`
to start. The file is gitignored because it is per device.

## Printing

Pick a printer in the designer. Five transports:

| Spec | Use |
|---|---|
| `browser` | Web Serial, straight from the page. Nothing installed on the device. |
| `tcp:192.168.1.50:9100` | A networked ESC/POS printer. |
| `usb:/dev/usb/lp0` | A USB printer on Linux, via the usblp character device. |
| `win:Receipt` | A Windows spooler queue, raw datatype. |
| `lp:Receipt` | A CUPS queue on macOS or Linux. |

`browser` is the one that makes a hosted deployment work. Web Serial needs
HTTPS, or localhost, and Chrome or Edge.

Press **Print test slip** before trusting a new printer. It costs 90mm of paper
and proves the three things a printer can silently fail at: reversed video, the
PC437 block glyphs the gauges are drawn from, and the native QR command.

## Events

Each conference is one JSON file. The receipt headline, logo, divider
characters, call to action, hiring line, share URL and kiosk palette all come
from it, so a new event is a file rather than a code change.

Defaults are compiled into the binary. Anything in `events/` overrides them by
code. Use the **New event** form in the designer to write one, or copy
`events/example-event.json` by hand.

## Deploying to a Windows kiosk

Run `run.bat` once. It installs Go if needed, builds, and starts the stand.

After that there are two things to open:

| | What it does |
|---|---|
| `kiosk.bat` | The stand. Locked to one full-screen tab, keeps itself alive. |
| `preview.bat` | The receipt designer, in a window you can close. |

Both run the same `kiosk.exe`, which starts the server, restarts it if it exits, and
reopens the app if it is closed. `scripts\autostart.bat`, run once as administrator, makes
it come back after a reboot.

Flags, if you need them: `-windowed` opens an app window instead of locking the
screen, `-once` exits rather than reopening, `-url` overrides the address.

For a hosted deployment, only `kiosk.exe` and a `roaster.json` holding
`kioskUrl` need to be on the device.

## Credentials

No model key ever goes on a kiosk device. With `"provider": "remote"` the
device posts a handle to `remoteUrl` and relays what comes back. The Anthropic
and GitHub keys live on that service, read from its environment. See
`.env.example`.

The device carries one credential: a terminal token, stored in `terminal.token`
beside the binary and encrypted with Windows DPAPI so the file is useless on
another machine. Write it with `roaster -set-token`, which reads from standard
input to keep it out of shell history. It is gitignored and never appears in a
settings file.

That token should be worth nothing to steal: scope it to the roast endpoint
alone, rate limit it, expire it with the event, and issue one per terminal so a
lost device is one revocation rather than a rotation. DPAPI does not stop
someone who boots the kiosk account, which auto-login hands them, so revocation
is the real protection and encryption is defence in depth.

## Regenerating the logo

```sh
python3 tools/svg2raster.py logo.svg 144 internal/ui/assets/logo.png
```

Thermal paper has one ink, so the SVG is flattened, filled with the even-odd
rule, supersampled and thresholded to 1 bit.

## Licence

MIT for the source code, see [LICENSE](LICENSE).

Two things in this repository are not covered by it. `internal/ui/fonts/integral-*.woff2`
are Fontspring demo builds of Integral CF, a commercial typeface, included so
the kiosk renders as designed on our own hardware. The Upgaming name and mark
are trademarks. Replace both, along with `internal/ui/assets/logo.png` and the
event files, before using this for anything of your own.
