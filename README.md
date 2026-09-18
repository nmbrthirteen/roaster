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

A packaged install cannot keep them there, because Windows mounts an MSIX
read-only. So the app writes wherever it can: the folder it runs from when that
is writable, which is the copied folder on a stand, and `%LOCALAPPDATA%\Roaster`
when it is not, which is every packaged install. Settings, event files, logs and
the terminal token all follow it, and the first line of each log says which
folder won. Whatever the package carries is copied out there on first run, so
the `roaster.json` that goes into the package is what a fresh device starts
with.

`addr` binds to loopback. A stand sits on venue wifi, and anything reachable
there could drive the printer. Widen it only for a deliberately hosted setup.

## Printing

Pick a printer in the designer. Five transports:

| Spec | Use |
|---|---|
| `browser` | Web Serial, straight from the page. Nothing installed on the device. |
| `tcp:192.168.1.50:9100` | A networked ESC/POS printer. |
| `usb:/dev/usb/lp0` | A USB printer on Linux, via the usblp character device. |
| `win:Receipt` | A Windows spooler queue, raw datatype. |
| `lp:Receipt` | A CUPS queue on macOS or Linux. |

`browser` is the one that makes a hosted deployment work: a page in Chrome or
Edge, served over HTTPS or from localhost, driving the printer itself.

The Windows app is not that. It draws the page in WebView2, which has no
`navigator.serial`, so it prints through the server beside it: `win:` for a
spooler queue, `tcp:` for a networked printer. The Web Serial option disappears
from the menu when the page is running somewhere that cannot do it.

Press **Print test slip** before trusting a new printer. It costs 90mm of paper
and proves the three things a printer can silently fail at: reversed video, the
PC437 block glyphs the gauges are drawn from, and the native QR command.

## Windows kiosk mode

`scripts\lockdown.bat` locks the machine around the stand by replacing the
shell, which needs no packaging and works on every edition.

To pick the app in Settings, Accounts, Set up a kiosk instead, it has to be an
MSIX. That picker lists Microsoft Edge and installed packaged apps and nothing
else, so a plain executable can never appear in it. Run `scripts\package.bat` as administrator. If `makeappx` and `signtool` are
not on the machine it fetches them itself, as a 22MB package rather than a
multi-gigabyte SDK install.

It builds both binaries, lays out the package, makes a self-signed certificate,
trusts it on that machine, signs, and installs. "Upgaming Roaster" then appears
in the kiosk picker, and in Start, and it launches itself at sign-in.

Assigned Access needs Windows 11 Pro or Enterprise; check with `winver`.

Once it is packaged, the log to read is `%LOCALAPPDATA%\Roaster\kiosk.log`, and
the settings to edit are beside it.

### Putting it on a device with no terminal

Package once on a machine that has Go and the SDK, then copy two files out of
`build\` to the kiosk. Both install by double clicking, no command line, no Go,
no SDK:

1. `Upgaming.cer`, install to **Local Machine**, then **Trusted People**.
2. `UpgamingRoaster.msix`, then Install.

## The hidden menu

A device locked to this one app still has to be serviceable, so everything an
operator would otherwise open Windows for is in the kiosk.

Type the code into the username field and press the arrow. It opens the menu
instead of running an audit. Tapping the mark five times within three seconds
brings up a keypad, which is the way in from a screen with no input on it.

Five wrong codes and the menu stops opening for a minute. A username that is
not a run of four to eight digits never costs a round trip.

The code is `adminPin` in `roaster.json`. It ships as **1379**, which is in a
public repository and therefore worth nothing. Change it before an event: the
menu can reboot the machine.

It shows the terminal, event, printer, wireless state, roasts printed, uptime
and the last error, and lets you choose a printer and connect it, send a test
slip, reprint the last receipt, scan and join a wireless network, switch or
create an event, set the terminal number, restart the app, exit the kiosk,
reboot, or shut down.

Anything that would end the event asks twice and forgets after five seconds, so
a stray touch cannot reboot the stand.

## Events

Each conference is one JSON file. The receipt headline, logo, divider
characters, call to action, hiring line, share URL and kiosk palette all come
from it, so a new event is a file rather than a code change.

Defaults are compiled into the binary. Anything in `events/` overrides them by
code. Use the **New event** form in the designer to write one, or copy
`events/example-event.json` by hand.

## The Windows app

`kiosk.exe` is the stand. It is a Windows application with its own window, icon
and taskbar identity, and it draws the kiosk page itself: no browser to close,
no address bar to reach, one thing to open. It starts `roaster.exe` beside it
and keeps the screen up.

| What goes wrong | What it does |
|---|---|
| The server exits | Starts it again, three seconds later. |
| The server is not up yet | Holds a waiting screen, on brand, until it answers. |
| The page stops running | Opens it again, within twenty seconds. |
| The view itself dies | Builds a new window, after ninety seconds. |
| Windows tries to sleep the screen | Refused for as long as the app runs. |
| Something else takes the screen | Back on top within the second. |

Full screen, always on top, no context menu, no zoom, no developer tools. It
swallows Alt+Tab, Alt+F4, Alt+Esc, Ctrl+Esc, Ctrl+Shift+Esc and the Windows key,
and leaves ordinary typing alone, because a visitor types a handle and an
operator types a code. Ctrl+Alt+Delete is not a shortcut a program can take, so
the lockdown script empties the screen it opens instead.

It needs the Microsoft Edge WebView2 Runtime, which Windows 11 ships and Edge
keeps updated on Windows 10.

Run `run.bat` once. It installs Go if needed, builds, and starts the stand.
After that:

| | What it does |
|---|---|
| `kiosk.bat` | The stand: full screen, locked, keeps itself alive. |
| `preview.bat` | The receipt designer, in a window you can close. |
| `stop.bat` | Stops everything, for when the hidden menu is not reachable. |

Flags, if you need them: `-windowed` opens a window that closes instead of
locking the screen, `-cursor` keeps the mouse pointer, `-url` overrides the
address, `-shell` tells it Windows started it in place of the desktop.

For a hosted deployment, only `kiosk.exe` and a `roaster.json` holding
`kioskUrl` need to be on the device.

## Locking the device to it

`scripts\lockdown.bat`, run once as administrator, gives the whole account to
the stand:

1. `kiosk.exe` replaces the desktop for that account. No taskbar, no start menu,
   nothing else to open, because nothing else is started.
2. The account signs in by itself, so a power cut ends with the stand back up.
3. Windows starts the shell again whenever it exits, which is the watchdog for a
   crash at four in the morning.
4. Task manager, the lock screen, password changes and notifications are off.
5. The screen never sleeps, the disk never spins down, USB never suspends, and
   Windows Update never reboots under a visitor.

It asks for a nightly reboot time on the way through; Enter means never. Two
things it cannot set, both firmware: restoring power state after a cut, and
booting with no keyboard attached.

Automatic sign-in as it stands needs a blank password on that account. Windows
stores a real one in clear text, so either clear the password or use Sysinternals
Autologon, which keeps it in the LSA secret store.

Three ways back out: **Exit kiosk** in the hidden menu, which starts the desktop;
`scripts\unlock.bat`, which undoes all of it; or holding Shift while signing in,
which skips the automatic sign-in.

`scripts\autostart.bat` is the lighter version: the stand starts at sign-in and
Windows is otherwise untouched.

## Credentials

No model key ever goes on a kiosk device. With `"provider": "remote"` the
device posts a handle to `remoteUrl` and relays what comes back. The Anthropic
and GitHub keys live on that service, read from its environment. See
`.env.example`.

The device carries one credential: a terminal token, stored in `terminal.token`
in the folder above and encrypted with Windows DPAPI so the file is useless on
another machine. Write it with `roaster -set-token`, which reads from standard
input to keep it out of shell history. A packaged install needs the token put
where it reads from, as the same Windows user:

```
roaster.exe -set-token -state %LOCALAPPDATA%\Roaster
``` It is gitignored and never appears in a
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
