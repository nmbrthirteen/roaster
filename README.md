# Roaster

A conference kiosk that reads a developer's public GitHub activity and prints
them a roast on thermal paper. Built for the Upgaming stand: a visitor types
their handle, the audit runs while they watch, and an 80mm receipt comes out
with a QR code to the digital version and the open roles.

Three Go binaries, none with a runtime to install. The front end is plain HTML,
CSS and JavaScript with no build step.

| | Runs on | What it is |
|---|---|---|
| `kiosk` | the stand | The Windows application a visitor sees. Starts `roaster` and shows its page. |
| `roaster` | the stand | The page, the receipt designer, the hidden menu and the printer. |
| `roastd` | a server | The roast service: reads GitHub, has the verdict written. The only place the keys live. |

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

`scripts\kioskmode.bat`, as administrator, locks the device to the stand with
Windows' own kiosk mode, Assigned Access. It builds both binaries, installs them
to `%ProgramFiles%\Roaster`, and names `kiosk.exe` in the Assigned Access
configuration. Restart and the device comes up on the stand, above the lock
screen, with the Assigned Access lockdown policies on. Windows starts the app
again whenever it closes.

| Command | What it locks |
|---|---|
| `scripts\kioskmode.bat` | An account Windows makes and signs in by itself after every restart. |
| `scripts\kioskmode.bat Stand` | The existing standard account `Stand`. Administrators are refused. |
| `scripts\kioskmode.bat off` | Nothing. The device signs in to Windows again after a restart. |

It needs Windows 11 21H2 or later, on Pro, Enterprise, Education or IoT
Enterprise. Earlier versions can run only a store app or Edge as the kiosk.
`scripts\check.bat` names the edition and says whether the configuration is set.
Ctrl+Alt+Del leaves the kiosk.

Run it again to ship a new build if you have a keyboard and admin rights on the
device. It stops the running stand, copies the new binaries over the old ones,
and sets the same configuration again. The hidden menu is the route without
either; see Updating a locked stand below.

The picker in Settings, Accounts, Set up a kiosk never lists this app. It offers
Edge and store apps, and `Set-AssignedAccess` takes the same two. A desktop
application counts as neither, even inside an MSIX. The configuration that does
take one, `KioskModeApp v4:ClassicAppPath`, goes in through the MDM bridge, and
the bridge answers only to SYSTEM. So `scripts\kioskmode.ps1` runs itself a
second time as SYSTEM for that step, through a scheduled task it deletes
afterwards.

Settings, events, logs and the terminal token live in `%ProgramFiles%\Roaster`,
beside the binaries. A standard account cannot normally write to Program
Files, so the install script grants the kiosk account Modify rights on that
folder; that is also what lets the hidden menu update the stand in place. The
log to read is `kiosk.log` in the same folder.

Do not combine it with `scripts\lockdown.bat`. Both sign an account in by
itself and both decide what it runs. Run `scripts\unlock.bat` before switching
to kiosk mode.

### Updating a locked stand

Push a tag and `.github/workflows/release.yml` builds and publishes it, in
about 3 minutes:

```
git tag v1.4.0
git push origin v1.4.0
```

On the stand, open the hidden menu (five taps on the mark, then the code), go
to Software, and press Check for updates. If a newer build is out, press
Install. A server-only change restarts the app in seconds; a change to
`kiosk.exe` reboots the device, since that is the window doing the showing.

If the new `roaster.exe` fails to start three times in a row, the launcher
puts the previous build back by itself, no operator required.

Local builds report version `dev` and always see a release as newer, since a
build made straight from source has no version of its own to compare.

### The MSIX

`scripts\package.bat` still builds a signed MSIX, for putting the app in Start
on a device that is not locked down. Kiosk mode does not use it. If the package
is installed on a kiosk, remove it, because its startup task races the kiosk for
the screen. `scripts\kioskmode.bat` warns when it finds one.

To put the MSIX on a device with no terminal, package once on a machine with Go,
then copy two files out of `build\`. Both install by double clicking:

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

`kiosk.exe` is the stand. It is an ordinary Windows application with its own
window, icon and taskbar identity, and it draws the kiosk page itself: no
browser to close, no address bar to reach, one thing to open. It starts
`roaster.exe` beside it and keeps that running.

It fills the screen and has no title bar, because a visitor has no use for one.
It is an ordinary window underneath: Alt+Tab reaches it, Alt+F4 closes it, and
nothing holds it in front of anything else. Holding the screen is kiosk mode's
job, and it does that job to whatever application it is given. `-window` opens
it with a title bar instead, which is what the designer does.

| What goes wrong | What it does |
|---|---|
| The server exits | Starts it again, three seconds later. |
| The server is not answering | Holds a waiting screen, on brand, until it does. |
| The server comes back | Opens the page again. |
| The page loses the server | Reloads itself, which the page has always done. |

No context menu, no zoom, no developer tools, and the built-in error page is off
so a server that is down reads in our words rather than Microsoft's. Nothing
else is taken away.

It needs the Microsoft Edge WebView2 Runtime, which Windows 11 ships and Edge
keeps updated on Windows 10.

Run `run.bat` once. It installs Go if needed, builds, and starts the stand.
After that:

| | What it does |
|---|---|
| `kiosk.bat` | The stand. |
| `preview.bat` | The receipt designer, at a size you can work in. |
| `stop.bat` | Stops everything, for when closing the window is not enough. |

Flags, if you need them: `-preview` opens the designer, `-window` gives it a
title bar and a size rather than the screen, `-url` overrides the address,
`-shell` tells it Windows started it in place of the desktop, so leaving puts
the desktop back.

For a hosted deployment, only `kiosk.exe` and a `roaster.json` holding
`kioskUrl` need to be on the device.

## Locking the device to it

`scripts\lockdown.bat`, run once as administrator, gives the whole account to
the stand:

1. `kiosk.exe` replaces the desktop for that account. No taskbar, no start menu
   and nothing else running, because nothing else is started. The app itself
   stops nobody reaching Windows, so this route is weaker than kiosk mode and
   is here for editions that have no kiosk mode.
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

## Reading GitHub

`internal/github` reads one account in one request. A stand has a queue in front
of it, and the difference between one round trip and five is the difference
between a visitor watching the screen and a visitor watching the floor. REST
would need a call for the profile, one for the repositories, one per repository
for commits and one for the contribution calendar. The GraphQL query answers all
of it at once, and the handle travels as a variable rather than as text spliced
into the query.

Two details in there are load-bearing. Commit timestamps keep the offset they
were made in rather than being normalised to UTC, which is what makes "three in
four of your commits happen after midnight" a fact about the person instead of a
fact about a timezone. And commits by other people are dropped, because somebody
else's commit message is theirs to answer for, unless GitHub attributed nothing
in that repository at all, in which case the owner keeps the lot.

The same request lists the top level of every repository it reads, which is how
a missing README is counted. A repository only counts as having none when that
is certain: a README of any spelling at the top, or a `.github` or `docs` folder
GitHub would also look in, and it has one. For an account with little else
public, the missing READMEs are often the truest line on the receipt, so the
rule errs toward never accusing anyone on a guess.

`internal/metric` turns that into the five rows the receipt is laid out for.
Every one of them is arithmetic over fetched facts. Nothing is estimated and
nothing is asked of a model, because a receipt someone photographs and shows to
the person next to them has to survive being checked.

See it on a real account:

```sh
export GITHUB_TOKEN=...   # no scopes needed; everything read is public
go run ./cmd/roaster -facts torvalds
```

It prints the gauges, the score and how long GitHub took. The token is read from
the environment and nowhere else: a flag would put it in shell history and in
the process list of a machine other people use.

## The roast service

`cmd/roastd` is the real roaster, and the only place the model key and the GitHub
token exist. A stand posts a handle with its terminal token and gets the audit
back as a stream. The kiosk's `remote` provider already speaks it, so pointing a
stand at the service is two settings:

```json
{ "provider": "remote", "remoteUrl": "https://roast.example.com/roast" }
```

and the terminal's token, written with `roaster -set-token`.

Run it with the settings in `.env.example` set in the environment:

```sh
go run ./cmd/roastd
```

### What it does with a handle

1. **Reads the account** in four GraphQL requests sent at once, so a busy
   account stays under GitHub's ten seconds a request.
2. **Measures it.** The five gauges, the score and the action items are arithmetic, and
   they reach the screen the moment GitHub answers.
3. **Asks for the verdict.** A model writes the one line that is not a number,
   at low effort because a queue is waiting: OpenAI's `gpt-5.6-luna` when
   `OPENAI_API_KEY` is set, or Claude Opus 5 when only an Anthropic key is. Every
   OpenAI request goes with `store` off, so a visitor's account is not kept on
   OpenAI's side past the reply.
4. **Prints regardless.** A verdict that is slow, refused, unprintable or never
   asked for is written from the numbers instead, and that line is true because
   it only says what was measured. The only thing that ends an audit early is
   GitHub itself, since without the account there is nothing true to print.

### Holding up under a crowd

- **A repeat costs GitHub nothing and still gets a new joke.** An account is
  read once and kept for ten minutes; the verdict is written fresh every time. A
  visitor who walks away mid-audit is usually back a second later, so the audit
  runs to the end and their account is already read when they return.
- **A crowd costs one read.** Five stands typing the speaker's handle at the same
  moment share a single GitHub request.
- **Load is bounded.** Sixteen audits run at once across every stand. A request
  that cannot get a slot within ten seconds is told the service is busy, rather
  than left hanging, and every audit is capped at forty seconds.
- **Each stand is limited.** A burst of six, then one every three seconds, which
  no queue of humans reaches and a stolen token does.
- **It keeps nothing it cannot lose.** Scale it by running more copies behind a
  load balancer. A deploy lets running roasts finish before it stops.
- **Every audit logs its cost.** GitHub reports each query's cost and the
  remaining hourly budget in the same response, so the log shows how close the
  token is to its limit rather than leaving it to be estimated.

### What keeps it safe

- **The model sees the work and nothing about the person.** No name, bio,
  employer, location or follower count, and not even the handle. It cannot make
  a joke about what it was never given.
- **Account text is treated as hostile.** Commit messages and repository names
  are written by whoever wants to write them, including someone hoping to make
  the stand say something it should not. They are capped, stripped of control
  characters, and fenced into a block the model is told is data; angle brackets
  are swapped for lookalikes, so nothing can close that block early.
- **What comes back is checked.** Links and swearing never reach paper. The
  model's line is thrown away and the numbers write it instead, so a false alarm
  costs a tamer joke and nothing more.
- **A refusal stays a refusal.** If the model declines a roast, the numbers
  write the tamer line. No second model is asked to try again.
- **Never the same joke twice.** The model is shown what this account was told
  on earlier visits and the last twelve lines the queue read, and told to share
  none of their jokes. The prompt also rules out the stock lines that fit anyone.
  When the numbers write the line instead, each gauge has several to rotate
  through, so the fallback does not repeat either.
- **Anyone can opt out.** `ROAST_OPT_OUT` lists handles that are never read at
  all.
- **Stands prove who they are.** Terminal tokens are compared as digests in
  constant time, and the log names a stand by a fingerprint of its token, never
  the token.

## Credentials

No model key ever goes on a kiosk device. With `"provider": "remote"` the
device posts a handle to `remoteUrl` and relays what comes back. The model key
and the GitHub token live on `roastd`, read from its environment, and on no
machine a visitor can touch. See `.env.example`.

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
