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

```sh
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o roaster.exe ./cmd/roaster
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o kiosk.exe ./cmd/kiosk
```

Copy `roaster.exe`, `kiosk.exe`, `roaster.json` and `events/` to the device.
`kiosk.exe` opens the app in Edge locked to one full-screen tab, with touch
gestures that would navigate away disabled.

For a hosted deployment, only `kiosk.exe` and a `roaster.json` holding
`kioskUrl` need to be on the device.

## Environment

Roast generation reads its credentials from the environment. See
`.env.example`. These belong wherever the app is hosted, never on a kiosk
machine standing in a public hall.

## Regenerating the logo

```sh
python3 tools/svg2raster.py logo.svg 144 internal/ui/assets/logo.png
```

Thermal paper has one ink, so the SVG is flattened, filled with the even-odd
rule, supersampled and thresholded to 1 bit.

## Licence

MIT, see [LICENSE](LICENSE). The Upgaming name and mark are trademarks and are
not covered by that licence. Replace `internal/ui/assets/logo.png` and the
event files with your own before using this for anything.
