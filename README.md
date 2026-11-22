# kapila-maker

**Malagasy (Plateau)** | [English](README.en.md) | [Français](README.fr.md)

[![CI](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml/badge.svg)](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

`kapila-maker` no anaran'ny rindranasa; mitazona ny anarana `kapila-burner` ny tahiry GitHub.

Fitaovana eo an-toerana hanamboarana **DVD-Audio LPCM** sy **Video CD 2.0**.
Misafidiana ny rindranasa GTK na ny fampiasana amin'ny mpitety tranonkala.
Tsy mila tolotra an-tserasera izy ireo ary mampiasa motera iray ihany.

## Alaina

Havaozina rehefa mahomby ny CI eo amin'ny sampana fototra ny kinova andrana
**continuous**.

**Mbola miandry fanamarinana CI ny fonosana vaovao.** Ny kinova efa navoaka
ankehitriny dia ilay `kapila-burner` taloha misy rakitra telo, mbola mila fitaovana
fanamboarana kapila ivelany. Tsy nahomby ny CI native voalohany ary mbola andramana
ny pipeline vaovao ho an'ny utilities. **Mbola miandry ireo rakitra vaovao fito
`kapila-maker` mandra-pahombiazan'ny CI sy fanamafisana fa efa navoaka izy fito.**
Mety hamerina 404 ireo rohy vaovao mandra-pahatongan'izany. Momba ny fonosana vaovao
ny fanazavana Windows/macOS eto ambany, fa tsy porofo fa efa voamarina na navoaka.

Installer (mbola miandry famoahana):
[Windows EXE](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64-setup.exe)
| [macOS Intel DMG](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64.dmg)

Arisiva portable hafa (anarana vaovao, mbola miandry famoahana):

| Rafitra | Rakitra alaina |
| --- | --- |
| Linux x86-64 | [kapila-maker-linux-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-linux-amd64.tar.gz) |
| Windows x86-64 | [kapila-maker-windows-amd64.zip](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64.zip) |
| macOS Intel x86-64 | [kapila-maker-macos-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64.tar.gz) |

[SHA256SUMS](https://github.com/radomd92/kapila-burner/releases/download/continuous/SHA256SUMS)
| [Pejin'ny kinova](https://github.com/radomd92/kapila-burner/releases/tag/continuous)
| [CI sy ny vokatra fanamboarana](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

Havaozina isaky ny CI mahomby manaraka
ny `continuous`. Ny pull request kosa dia manana artifacts ao amin'ny Actions,
tehirizina **14 andro**, fa tsy mamoaka kinova ho an'ny daholobe.
Tsy midika ho fahombiazan'ny fanamboarana rehetra ny fisian'ireo rohy ireo;
jereo ny satan'ny Actions.

Ao anatin'ny arisiva tsirairay ny **`kapila-maker`** (GTK) sy
**`kapila-maker-web`** (web); misy tovana `.exe` amin'ny Windows.
Vahao aloha ny arisiva. Tsy mila GTK ny kinova web.
Ny tar.gz macOS vaovao dia misy **`kapila-maker.app`** sy ireo executable
GTK/web ho fanamorana ny fampiasana. Ao amin'ny fonosana Windows/macOS vaovao
ny GTK, ny libraries tsy avy amin'ny rafitra sy angon-drakitra rehetra ilaina,
ary ny fitaovana dimy fanamboarana image ho an'ny interface roa.

| Rafitra | Zavatra ilaina amin'ny GTK |
| --- | --- |
| Linux | Ubuntu 22.04 na vao haingana kokoa, na rafitra mifanaraka aminy; GTK 3 avy amin'ny rafitra (`libgtk-3-0` amin'ny Ubuntu) sy tontolo an-tsary. |
| Windows | Ampiasao ny installer EXE, na tazomy ny **lahatahiry ZIP voavaha manontolo** ary alefaso ny `kapila-maker.exe` / `launch-gtk.cmd`. Ao ny GTK DLL, libraries sy angon-drakitra ilaina; aza afindra irery ny `.exe`. |
| macOS | Intel macOS 15 na vao haingana kokoa. Sokafy ny DMG ary tariho ao amin'ny Applications ny **`kapila-maker.app`** mahaleo tena, na ampiasao ny tar.gz. Ao ny GTK, libraries sy angon-drakitra ilaina; tsy mila Homebrew. |

Tsy misy kinova ARM avoaka. Amin'ny Linux ihany no mbola ilaina ny GTK avy
amin'ny rafitra sy ny fitaovana fanamboarana kapila apetraka misaraka.
**Linux ihany** no manohana fandoroana VCD ara-batana, miaraka amin'ny `cdrdao`
tsy voatery apetraka raha image ihany no atao, sy fanekena mazava.
Tsy tohanana ny fandoroana amin'ny Windows/macOS: mampiasà rindranasa fandoroana
ivelany ho an'ny ISO na BIN/CUE. **Tsy ilaina izany mba hamoronana image**, ary
tsy misy fandoroana mandeha ho azy.

Ao amin'ny `%LOCALAPPDATA%\Programs\kapila-maker` ho an'ilay mpampiasa ny
installer Windows, tsy mila admin, ary misy shortcuts sy uninstaller.
**Tsy misy sonia** izy: mety haneho fampitandremana ny Windows/SmartScreen;
tsy lazaina ho manana code-signing certificate. Ny macOS app dia manana
**sonia ad hoc, fa tsy Developer ID ary tsy notarized**. Araho ny fitsipika
mahazatra Gatekeeper sy ny an'ny fikambananao; aza vonoina ny fiarovana maneran'ny
rafitra. Tsy mila Homebrew, MSYS2, Go, Python, shell, compiler, fametrahana
fitaovana misaraka na fanamboarana `PATH` ny mpampiasa Windows/macOS vaovao.
Ho an'ny fanamboarana CI ihany ireo fitaovana fampandrosoana.

Rehefa avoaka ny vaovao, mandrakotra **rakitra fito** ny `SHA256SUMS`: arisiva
portable telo, installer roa (EXE/DMG), ary arisiva source roa.
Ny SHA-256 dia hanamarinana ny fitovian'ny rakitra alaina amin'ny `SHA256SUMS`,
fa **tsy sonia kriptografika**. Jereo ny [torolalana fanamarinana](README.en.md#download).

## Fanombohana

Amin'ny Windows, ampiasao ny shortcut napetraka; amin'ny macOS, sokafy ny
`kapila-maker.app` ao amin'ny Applications. Tsy mila terminal ireo fomba GTK
ireo. Raha mampiasa baiko kosa, ao amin'ilay lahatahiry voavaha, Linux na macOS:

```sh
./kapila-maker -root /path/to/media
# Raha tiana ny teny anglisy na frantsay:
./kapila-maker -root /path/to/media -lang en
./kapila-maker -root /path/to/media -lang fr
# Na ampiasao ny mpitety tranonkala:
./kapila-maker-web -root /path/to/media
```

Amin'ny ZIP portable Windows, sokafy ny `kapila-maker.exe` / `launch-gtk.cmd`,
na alefaso ao amin'ny PowerShell:

```powershell
.\launch-gtk.cmd -root C:\Music
# Ho an'ny web:
.\kapila-maker-web.exe -root C:\Music
```

Soloy amin'ny toerana misy ny haino aman-jerinao ny `-root`.
Ny fonosana Windows/macOS vaovao dia mitady fitaovana mifototra amin'ny toerana
misy ny executable, ka mandeha avy amin'ny **lahatahiry iasana rehetra**.
Ny Music an'ny mpampiasa no tahiry media fanombohana; ao amin'ny cache an'ilay
mpampiasa, `kapila-maker/output`, ny vokatra. Manolo ireo ny `-root` sy `-output`
raha voafaritra. Tsy miova ny source/Linux: `..` ho an'ny media, `./output`
ho an'ny vokatra, ary fikarohana fitaovana eo an-toerana araka ny lahatahiry iasana.
Amin'ny web, sokafy ilay URL aseho, matetika
**http://127.0.0.1:8787**, ary avelao handeha ny terminal. `Ctrl-C` no mampijanona azy.
Aza aseho amin'ny tambajotra ny server ary aza alefa amin'ny `sudo` ny rindranasa.

Malagasy (`plt`) no teny fanombohana amin'ny GTK sy amin'ny web vaovao.
Azo ovaina eo am-piasana ho English (`en`) na Français (`fr`). Ny web dia
mitahiry ny safidy ao amin'ny navigateur: mbola hajaina ny teny anglisy efa
nofidina sy voatahiry. Ny anaran'ny tetikasa, vondrona ary rakitra efa misy
dia tsy ovaina. Ny diarin'asa sy ny hadisoana novokarin'ny rindranasa dia
mitazona ny teny nampiasaina tamin'ny nanombohan'ilay asa; tsy adika ny
hafatra manta avy amin'ny fitaovana ivelany.

## Fitaovana Ivelany

Ao amin'ny **fonosana Windows/macOS vaovao, mbola miandry CI sy famoahana**,
ny fitaovana dimy namboarina avy amin'ny source: FFmpeg/FFprobe **9.0.2**,
VCDImager **2.0.1**, ny fototra `dvda-author`, ary `mkisofs` tena manohana
DVD-Audio. Ao amin'ny [utility sources manifest](scripts/utility-sources.tsv)
ny version/commit voafetra, URL sy SHA-256. Amin'ny **Linux**, mbola apetraka
misaraka araka ny karazana kapila ireo fitaovana dimy; tsy voatery ny `cdrdao`
raha tsy mandoro VCD. Jereo ao amin'ny rindranasa ny satan'ny fitaovana.

| Fampiasana | Fitaovana (apetraka misaraka amin'ny Linux) |
| --- | --- |
| Famakiana sy fanovana haino aman-jery | FFmpeg sy FFprobe |
| DVD-Audio | `dvda-author`; `mkisofs` tena manohana `-dvd-audio` raha mamorona ISO. Tsy ampy ny `genisoimage` tsy manana izany safidy izany. |
| Video CD | VCDImager (`vcdimager`); tsy mila fitaovana DVD. |
| Fandoroana VCD amin'ny GTK, **Linux ihany** | `cdrdao`, mpandoro CD ary CD-R/CD-RW foana; mila fanekena mazava foana. |

Azo faritana amin'ny lalana feno ny `FFMPEG`, `FFPROBE`, `DVDA_AUTHOR`,
`MKISOFS`, `VCDIMAGER` ary `CDRDAO`. Jereo ny
[torolalana feno](README.en.md#dependencies) ho an'ny fikarohana fitaovana sy ny fetra.
Ny fitaovana ao amin'ny fonosana Windows/macOS dia hita mifototra amin'ny
executable, tsy mila fanamboarana `PATH`; ny Linux/source no mitazona ny
fikarohana `./tools/bin/` sy `PATH` teo aloha.

### Source Sy Fahazoan-dalana

Havoaka miaraka amin'ny vaovao ny
[`kapila-maker-windows-amd64-sources.tar.gz`](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64-sources.tar.gz)
sy
[`kapila-maker-macos-amd64-sources.tar.gz`](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64-sources.tar.gz),
mbola miandry famoahana koa. **Tsy mila ireo source ireo hampandehanana ny app.**
Ao ny arisiva source marina an'ireo formula runtime macOS, ny source rehetra
an'ireo package MSYS2 mifanaraka aminy, ny source utility tany am-boalohany,
patches, scripts sy licences. Tazomy ny filazana fahazoan-dalana; samy manana
ny licence-ny ny singa tsirairay. Ao amin'ny
[torolalana anglisy](README.en.md#corresponding-sources) ny antsipirian'ny
manifest sy ny fanamboarana utility tsy mila tambajotra, raha ilaina.

## Fampiasana

1. Mamorona tetikasa **DVD Audio** na **Video CD**, dia andraso ny fizahana ny tahiry.
2. Safidio sy ampio ireo rakitra, alamino ny vondrona sy ny filaharan'ny hira.
3. Hamarino ny tomban'ny haben'ny kapila sy ny toerana malalaka ilaina.
4. Safidio ny **Create Image** ao amin'ny GTK na ny bokotra fanamboarana ao amin'ny web.
5. Alao ny ISO DVD-Audio, na ny **`disc.cue` sy `disc.bin` miaraka** ho an'ny VCD.
   Tazomy ao amin'ny lahatahiry iray ireo rakitra VCD roa ireo, tsy ovaina anarana.

Ny DVD-Audio dia LPCM tsy voaporitra, **fa tsy DVD-Video na kapila MP3**;
mila mpamaky DVD-Audio mifanaraka aminy. Stereo 16-bit/44.1 kHz no fanombohana
tsara; araho ny tombana sy ny fanamarinana farany. Tsy mamerina kalitao very
ny PCM. Zarao mialoha ny hira mihoatra ny fetran'ny WAV manodidina ny 4 GiB.

Ny VCD 2.0 dia MPEG-1/MP2 PAL na NTSC, misy fahaverezan-kalitao, fa tsy Audio CD
na MP3 CD. Latsaky ny 74/80 minitra ny fotoana azo ampiasaina amin'ny CD74/CD80.

Tsy mandoro kapila ny web. Ny GTK amin'ny Linux ihany no afaka mandoro VCD,
**rehefa vita ny image tao amin'io session io ihany**, voafidy ny fitaovana
sy ny hafainganam-pandeha, ary nomena fanekena mazava. Tsy mandeha ho azy izany;
mety hanimba CD ny fanafoanana. Ny ISO DVD kosa dorana amin'ny rindranasa
ivelany amin'ny fomba **write disc image**, fa tsy adika ho rakitra amin'ny kapila.
Tsy antoky ny filalaovana amin'ny fitaovana rehetra ny fahombiazan'ny image.

## Tetikasa Sy Vokatra

Ao amin'ny GTK, ampiasao mazava ny **Save / Save As** (`Ctrl+S` / `Ctrl+Shift+S`).
Manamarika fanovana tsy voatahiry ny `*`; New, Open, Close ary Quit dia
manontany **Save / Discard / Cancel** mba hiarovana azy ireo. Tsy misy autosave GTK.
Ny web dia mitahiry ao amin'ny navigateur ihany; manaova Export ho an'ny tetikasa
manan-danja. Mitovy ny JSON ampiasain'ny GTK Open/Save sy ny web Import/Export.
Tsy backup an'ireo haino aman-jery ny JSON: mbola ilaina eo amin'ny lalany ny loharano.

Tsy ovaina ny loharano. Ao amin'ny `output/disc-*/` ny vokatra sy `project.json`,
`job.json` ary ny logs; ao amin'ny `output/burn-*/` ny asam-pandoroana.
Miankina amin'ny output root voafidy ireo lalana ireo: cache an'ny mpampiasa
`kapila-maker/output` amin'ny fonosana Windows/macOS vaovao, `./output` amin'ny
source/Linux raha tsy novaina tamin'ny flags.
Tazonina ny vokatra tsy feno, tsy misy fanadiovana mandeha ho azy na fitohizan'ny
asa aorian'ny restart. Ho an'ny DVD ISO, manomana toerana avo telo henin'ny PCM,
miampy elanelana fiarovana. Mampiasà haino aman-jery sy fitaovana azo itokisana:
tsy sandbox ny rindranasa.

## Ho An'ny Mpamorona

Ny [README anglisy](README.en.md) no torolalana teknika feno. Mila Go 1.22 na
vaovao kokoa ny source; Go 1.24 no ampiasain'ny CI. Avy ao amin'ny tahiry nakarina:

```sh
make web                 # kapila-maker-web
make gtk                 # kapila-maker, mila GTK development files
make test
make test-gtk
make smoke-gtk           # en, plt, fr; mila display na Xvfb
VCD_INTEGRATION=1 make test
# Safidy eo an-toerana, rehefa napetraka ny fitaovana DVD-Audio rehetra:
DVDA_INTEGRATION=1 VCD_INTEGRATION=1 go test -race ./...
```

`cmd/kapila-maker` no lalana source GTK; `kapila-maker` sy `kapila-maker-web`
no anaran'ny executable eo an-toerana sy ao amin'ny fonosana avoaka.
Ao amin'ny [tools/gtk-dev](tools/gtk-dev/README.md) ny overlay GTK ho an'ny
fampandrosoana amin'ny Linux sasany.

Ny CI dia manamboatra ny arisiva telo ary manao tests, vet, GTK smoke amin'ny
teny telo sy VCD integration amin'ny Linux. Ny CI bundling vaovao dia kasaina
hanamboatra utility avy amin'ny source amin'ny Windows/macOS, hamorona image
DVD-Audio sy VCD amin'ny `PATH` voadio, ary hizaha `.app` macOS nafindra toerana.
Tsy maintsy mijanona ho diso ny fanangonana source raha misy source mifanaraka
tsy hita. **Mbola miandry fanamarinana ireo fitsapana vaovao ireo.** Tsy porofo
ho an'ny fonosana vaovao ny fahombiazan'ny CI sy release taloha.
Tsy misy fandoroana na fitsapana playback ara-batana amin'ny CI. Amin'ny Linux
no anaovana ny fitsapana an-tsary GTK; mihoatra ny `-h` taloha ny fitsapana
image sy relocation vaovao, fa tsy manamarina fifandraisana an-tsary amin'ny
Windows/macOS. Jereo ny logs CI sy ny lisitry ny rakitra navoaka.
Manaova sampana avy amin'ny `main` ary pull request mankany
amin'ny `main` raha handray anjara.
