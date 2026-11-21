# Kapila Burner

**Malagasy (Plateau)** | [English](README.en.md) | [Français](README.fr.md)

[![CI](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml/badge.svg)](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

Fitaovana eo an-toerana hanamboarana **DVD-Audio LPCM** sy **Video CD 2.0**.
Misafidiana ny rindranasa GTK na ny fampiasana amin'ny mpitety tranonkala.
Tsy mila tolotra an-tserasera izy ireo ary mampiasa motera iray ihany.

## Alaina

Ny kinova andrana **continuous** dia avy amin'ny fanamboarana CI farany nahomby
teo amin'ny sampana fototra:

| Rafitra | Rakitra alaina |
| --- | --- |
| Linux x86-64 | [kapila-burner-linux-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-linux-amd64.tar.gz) |
| Windows x86-64 | [kapila-burner-windows-amd64.zip](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-windows-amd64.zip) |
| macOS Intel x86-64 | [kapila-burner-macos-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-macos-amd64.tar.gz) |

[SHA256SUMS](https://github.com/radomd92/kapila-burner/releases/download/continuous/SHA256SUMS)
| [Pejin'ny kinova](https://github.com/radomd92/kapila-burner/releases/tag/continuous)
| [CI sy ny vokatra fanamboarana](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

Nahomby ny [CI voalohany](https://github.com/radomd92/kapila-burner/actions/runs/44952181031)
ho an'ireo rafitra telo ary efa navoaka ireo rakitra alaina. Havaozina isaky ny CI mahomby manaraka
ny `continuous`. Ny pull request kosa dia manana artifacts ao amin'ny Actions,
tehirizina **14 andro**, fa tsy mamoaka kinova ho an'ny daholobe.
Tsy midika ho fahombiazan'ny fanamboarana rehetra ny fisian'ireo rohy ireo;
jereo ny satan'ny Actions.

Ao anatin'ny arisiva tsirairay ny **`kapila-burner`** (GTK) sy
**`kapila-burner-web`** (web); misy tovana `.exe` amin'ny Windows.
Vahao aloha ny arisiva. Tsy mila GTK ny kinova web.

| Rafitra | Zavatra ilaina amin'ny GTK |
| --- | --- |
| Linux | Ubuntu 22.04 na vao haingana kokoa, na rafitra mifanaraka aminy; GTK 3 avy amin'ny rafitra (`libgtk-3-0` amin'ny Ubuntu) sy tontolo an-tsary. |
| Windows | Efa ao ny GTK DLL sy ny angon-drakitra ilainy. Tazomy ny **lahatahiry manontolo**, ary ampiasao ny `launch-gtk.cmd`. |
| macOS | Namboarina ho an'ny Intel macOS 15; mila Homebrew GTK 3 ao amin'ny `/usr/local`: `brew install gtk+3`. Tsy `.app`, tsy mahaleo tena, **tsy misy sonia ary tsy notarized**. |

Tsy misy kinova ARM avoaka. Tsy ao anatin'ny arisiva ny fitaovana fanamboarana
kapila: apetraho misaraka ireo ilaina etsy ambany amin'ny rafitra rehetra.
Ny SHA-256 dia hanamarinana ny fitovian'ny rakitra alaina amin'ny `SHA256SUMS`,
fa **tsy sonia kriptografika**. Jereo ny [torolalana fanamarinana](README.en.md#download).

## Fanombohana

Ao amin'ilay lahatahiry voavaha, amin'ny Linux na macOS:

```sh
./kapila-burner -root /path/to/media
# Raha tiana ny teny anglisy na frantsay:
./kapila-burner -root /path/to/media -lang en
./kapila-burner -root /path/to/media -lang fr
# Na ampiasao ny mpitety tranonkala:
./kapila-burner-web -root /path/to/media
```

Amin'ny Windows, sokafy ny `launch-gtk.cmd`, na alefaso ao amin'ny PowerShell:

```powershell
.\launch-gtk.cmd -root C:\Music
# Ho an'ny web:
.\kapila-burner-web.exe -root C:\Music
```

Soloy amin'ny toerana misy ny haino aman-jerinao ny `-root`.
Ny lahatahiry iasana ankehitriny no ampiasaina hitadiavana fitaovana sy hametrahana
ny `output/` raha tsy voafaritra ny `-output`; alefaso ao amin'ny lahatahiry
misy ny rindranasa. Amin'ny web, sokafy ilay URL aseho, matetika
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

Tsy misy amin'ireto fitaovana ireto tafiditra ao amin'ny arisiva. Jereo ao
amin'ny rindranasa ny satan'ny fitaovana hita amin'ny solosainao.

| Fampiasana | Fitaovana apetraka misaraka |
| --- | --- |
| Famakiana sy fanovana haino aman-jery | FFmpeg sy FFprobe |
| DVD-Audio | `dvda-author`; `mkisofs` tena manohana `-dvd-audio` raha mamorona ISO. Tsy ampy ny `genisoimage` tsy manana izany safidy izany. |
| Video CD | VCDImager (`vcdimager`); tsy mila fitaovana DVD. |
| Fandoroana VCD amin'ny GTK, **Linux ihany** | `cdrdao`, mpandoro CD ary CD-R/CD-RW foana; mila fanekena mazava foana. |

Azo faritana amin'ny lalana feno ny `FFMPEG`, `FFPROBE`, `DVDA_AUTHOR`,
`MKISOFS`, `VCDIMAGER` ary `CDRDAO`. Jereo ny
[torolalana feno](README.en.md#dependencies) ho an'ny fikarohana fitaovana sy ny fetra.

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
Tazonina ny vokatra tsy feno, tsy misy fanadiovana mandeha ho azy na fitohizan'ny
asa aorian'ny restart. Ho an'ny DVD ISO, manomana toerana avo telo henin'ny PCM,
miampy elanelana fiarovana. Mampiasà haino aman-jery sy fitaovana azo itokisana:
tsy sandbox ny rindranasa.

## Ho An'ny Mpamorona

Ny [README anglisy](README.en.md) no torolalana teknika feno. Mila Go 1.22 na
vaovao kokoa ny source; Go 1.24 no ampiasain'ny CI. Avy ao amin'ny tahiry nakarina:

```sh
make web                 # dvd-audio-studio
make gtk                 # dvd-audio-gtk, mila GTK development files
make test
make test-gtk
make smoke-gtk           # en, plt, fr; mila display na Xvfb
VCD_INTEGRATION=1 make test
# Safidy eo an-toerana, rehefa napetraka ny fitaovana DVD-Audio rehetra:
DVDA_INTEGRATION=1 VCD_INTEGRATION=1 go test -race ./...
```

`cmd/dvd-audio-gtk` sy `dvd-audio-gtk` no anaran'ny source sy build eo an-toerana;
**Kapila Burner** / `kapila-burner` no marika sy anaran'ny executable avoaka.
Ao amin'ny [tools/gtk-dev](tools/gtk-dev/README.md) ny overlay GTK ho an'ny
fampandrosoana amin'ny Linux sasany.

Ny CI dia manamboatra ny arisiva telo ary manao tests, vet, GTK smoke amin'ny
teny telo sy VCD integration amin'ny Linux. **Tsy misy DVD-Audio integration
amin'ny hosted CI**, ary tsy misy fandoroana kapila ara-batana. Nahomby ny
fanamboarana sy famoahana ireo rindranasa roa ho an'ny rafitra telo.
Amin'ny Linux no anaovana ny fitsapana an-tsary GTK; `-h` ihany no andramana
amin'ny Windows sy macOS. Manaova sampana avy amin'ny `main` ary pull request mankany
amin'ny `main` raha handray anjara.
