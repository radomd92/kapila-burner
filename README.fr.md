# Kapila Burner

[Malagasy (Plateau)](README.md) | [English](README.en.md) | **Français**

[![CI](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml/badge.svg)](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

Un atelier local pour créer des disques **DVD-Audio LPCM non compressé** et des
images **Video CD 2.0**, avec une interface native GTK ou une interface web locale.
Les deux interfaces utilisent le même moteur, sans service en ligne.
La création d'une image précède toute gravure : rien n'est gravé automatiquement.

## Télécharger

Dernière compilation réussie de la branche par défaut, publiée en **préversion
continue** (`continuous`) :

| Plateforme | Archive |
| --- | --- |
| Linux x86-64 | [kapila-burner-linux-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-linux-amd64.tar.gz) |
| Windows x86-64 | [kapila-burner-windows-amd64.zip](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-windows-amd64.zip) |
| macOS Intel x86-64 | [kapila-burner-macos-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-macos-amd64.tar.gz) |

[SHA256SUMS](https://github.com/radomd92/kapila-burner/releases/download/continuous/SHA256SUMS)
| [Page de la préversion](https://github.com/radomd92/kapila-burner/releases/tag/continuous)
| [Workflow CI et artefacts](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

La [première exécution CI hébergée](https://github.com/radomd92/kapila-burner/actions/runs/44952181031)
a compilé les trois archives avec succès et publié ces téléchargements.
Chaque exécution réussie suivante actualise cette préversion. Les pull requests
produisent des artefacts Actions conservés **14 jours**, sans publication de
release. Les tags `v*` publient des versions distinctes.

Chaque archive contient **`kapila-burner`** (GTK natif) et
**`kapila-burner-web`** (web portable), avec l'extension `.exe` sous Windows.
Extrayez l'archive avant utilisation. Le programme web ne nécessite pas GTK.

| Plateforme | Exigences de l'interface GTK |
| --- | --- |
| Linux | Ubuntu 22.04 ou plus récent, ou distribution compatible, avec GTK 3 système (`libgtk-3-0` sous Ubuntu) et une session graphique. |
| Windows | Les DLL GTK et leurs données sont incluses. Conservez **tout le dossier extrait** et lancez `launch-gtk.cmd`, pas un `.exe` déplacé seul. |
| macOS | Compilation Intel macOS 15, liée à Homebrew GTK 3 dans `/usr/local` : `brew install gtk+3`. Ce n'est ni une application autonome ni un paquet `.app`. Les exécutables sont **non signés et non notariés**. |

Aucun exécutable ARM n'est publié. Les outils de conversion et d'authoring
doivent être installés séparément sur **chaque plateforme**. La gravure physique
de VCD est réservée à **Linux**, avec `cdrdao` et confirmation explicite.

Téléchargez `SHA256SUMS` pour vérifier l'intégrité des archives. Sous Linux :

```sh
sha256sum --check --ignore-missing SHA256SUMS
```

Sous macOS, comparez `shasum -a 256 kapila-burner-macos-amd64.tar.gz` à la valeur
publiée. Sous PowerShell, utilisez
`Get-FileHash .\kapila-burner-windows-amd64.zip -Algorithm SHA256`.
Ces sommes de contrôle **ne sont pas des signatures cryptographiques**.

## Démarrer

Depuis le dossier extrait, sous Linux ou macOS :

```sh
./kapila-burner -root /path/to/media
# Démarrer directement en français :
./kapila-burner -root /path/to/media -lang fr
# Ou en anglais :
./kapila-burner -root /path/to/media -lang en
# Autre interface : navigateur local
./kapila-burner-web -root /path/to/media
```

Sous Windows, ouvrez `launch-gtk.cmd` ou lancez depuis PowerShell :

```powershell
.\launch-gtk.cmd -root C:\Music -lang fr
# Interface web :
.\kapila-burner-web.exe -root C:\Music
```

Remplacez la racine par votre dossier de médias. Lancez depuis le dossier de
l'application : la recherche des outils et le dossier `output/` par défaut
dépendent du **répertoire de travail**, pas de l'emplacement de l'exécutable.
L'option `-output` permet de choisir un autre dossier de sortie.

Pour le web, ouvrez l'URL exacte affichée, normalement **http://127.0.0.1:8787**.
Gardez le terminal ouvert ; `Ctrl-C` arrête le serveur et annule le travail actif.
Ne publiez pas ce serveur sur un réseau, un proxy ou un tunnel. N'exécutez pas
l'application avec `sudo`.

## Langues Et Projets

Les deux interfaces proposent le malgache du Plateau (`plt`), l'anglais (`en`)
et le français (`fr`), avec changement de langue à chaud. GTK démarre en
malgache, sauf si `-lang en` ou `-lang fr` est indiqué. Une nouvelle interface
web démarre aussi en malgache ; elle conserve le choix dans le `localStorage`
du navigateur, y compris un choix anglais déjà enregistré.

Les noms existants de projets, de groupes et de fichiers restent inchangés.
Les journaux et erreurs produits par l'application gardent la langue choisie
au **démarrage du travail**, même après un changement de langue de l'interface.
Les diagnostics bruts des outils externes ne sont pas traduits.

Dans GTK, utilisez **Save / Save As** (`Ctrl+S` / `Ctrl+Shift+S`) pour enregistrer
explicitement : il n'y a pas d'enregistrement automatique. Un `*` signale les
modifications non enregistrées. New, Open, Close et Quit proposent
**Save / Discard / Cancel** pour protéger ces modifications. Les enregistrements
sont atomiques et Save réutilise le chemin choisi. `-project /path/to/project.json`
permet d'ouvrir un projet au lancement.

Le web conserve automatiquement le projet dans ce navigateur, mais son stockage
peut être effacé : exportez les projets importants. GTK Open/Save et web
Import/Export échangent le même JSON. Ce manifeste n'est **pas une sauvegarde
des médias** : les sources doivent rester aux mêmes chemins relatifs à la racine.

## Outils Externes

**Aucun des outils suivants n'est inclus dans les archives**, sur aucune
plateforme. Vérifiez leur disponibilité et leur compatibilité sur votre OS.
Leur état et leur chemin apparaissent dans l'interface.

| Fonction | Outils nécessaires |
| --- | --- |
| Analyse et conversion | FFprobe et FFmpeg |
| DVD-Audio | `dvda-author` ; un véritable `mkisofs` prenant en charge `-dvd-audio` pour créer l'ISO. Un substitut `genisoimage` sans cette option ne convient pas. |
| Video CD | VCDImager (`vcdimager`) ; les outils DVD ne sont pas nécessaires. |
| Gravure VCD GTK, Linux uniquement | `cdrdao`, un graveur CD et un CD-R/CD-RW vierge. |

Vous pouvez imposer des chemins absolus avec `FFMPEG`, `FFPROBE`, `DVDA_AUTHOR`,
`MKISOFS`, `VCDIMAGER` et `CDRDAO`. Sans surcharge, seuls `dvda-author` et
`mkisofs` sont recherchés dans `./tools/bin/` avant le `PATH` ; les autres
utilisent le `PATH`. Redémarrez l'interface après un changement d'outils.
Consultez le [guide détaillé](README.en.md#dependencies) pour les outils locaux
facultatifs et leurs limites de portabilité.

## Créer Un DVD-Audio

1. Choisissez **DVD Audio** et attendez l'analyse récursive de la bibliothèque.
2. Filtrez les fichiers, ajoutez les pistes et réglez leur ordre. Vous pouvez créer jusqu'à neuf groupes de 99 pistes ; les doublons sont autorisés.
3. Commencez avec **stéréo, 16 bits, 44,1 kHz**, DVD-5 et ISO activé. Vérifiez les estimations de capacité et d'espace de travail.
4. Lancez **Create Image** dans GTK ou **Build DVD-Audio** dans le web. Le moteur convertit en WAV, crée `AUDIO_TS`, vérifie les fichiers puis produit éventuellement l'ISO.
5. Récupérez l'ISO dans le dossier de sortie ou téléchargez-le depuis le web. Pour le graver, utilisez un logiciel externe en mode **graver une image**, pas en copiant le fichier ISO sur un disque de données.

Le DVD-Audio n'est **ni un DVD-Video ni un disque MP3** : il exige un lecteur
DVD-Audio compatible. Il n'y a ni menus visuels, ni MLP, ni multicanal, ni DRM.
Les noms de groupes sont des libellés de projet, pas des titres affichés par le lecteur.

Les fichiers compressés deviennent du PCM : leur taille d'origine ne prédit
pas l'occupation du disque, et le suréchantillonnage ne restaure pas la qualité
perdue. Comptez environ **sept heures stéréo sur DVD-5** à 16 bits/44,1 kHz,
sans garantie : fiez-vous à l'estimation et au contrôle final. Chaque piste
doit tenir dans la limite WAV classique d'environ **4 Gio** ; découpez les
enregistrements trop longs avant de les ajouter. Le
[guide anglais](README.en.md#dvd-capacity-and-long-tracks) donne un exemple FFmpeg.

## Créer Un Video CD

Choisissez **Video CD**, PAL ou NTSC, puis CD74/CD80. Ajoutez et ordonnez
**1 à 98 pistes** dans le groupe unique, vérifiez la capacité puis créez l'image.
Les sources restent intactes. Les entrées audio seules reçoivent une image
noire ; les vidéos sont adaptées au format 4:3.

Le résultat est un **VCD 2.0 MPEG-1/MP2 avec pertes**, pas un Audio CD ni un
CD MP3. PAL utilise 352×288 à 25 images/s ; NTSC, 352×240 à environ 29,97 images/s.
L'audio est MP2 stéréo à 44,1 kHz et 224 kbit/s. La durée utile est inférieure
aux 74/80 minutes nominales à cause des réserves et des intervalles.

Téléchargez **les deux fichiers `disc.cue` et `disc.bin`** et gardez-les ensemble,
avec ces noms. Le CUE référence le BIN relatif. Utilisez un logiciel comme K3b
en mode gravure d'image, pas en mode disque de données.

### Gravure VCD

Le web n'offre **aucune API de gravure**. Dans GTK sous **Linux uniquement**,
créez d'abord l'image puis choisissez **Burn VCD Image / Burn**. Seule une image
créée avec succès dans **la même session GTK** est admissible ; le moteur en
vérifie les empreintes SHA-256. Redémarrer l'application ne restaure pas cette
autorisation depuis `image.json`. Recréez l'image dans GTK ou gravez-la à l'extérieur.

Sélectionnez le graveur, un **CD-R/CD-RW vierge, jamais un DVD**, et une vitesse
de 1× à 52× (8× par défaut), puis confirmez explicitement. Le mode Dummy est
une simulation sur un vrai graveur avec `cdrdao`, si celui-ci la prend en charge,
et demande aussi confirmation. Annuler peut rendre le CD inutilisable.
Corrigez les droits d'accès au périphérique plutôt que de lancer avec `sudo`.
Une image valide ne garantit pas la lecture sur tout appareil.

## Sorties Et Sécurité

Chaque travail conserve son dossier `output/disc-*/`, avec `project.json`,
`job.json`, les intermédiaires et l'ISO DVD ou la paire BIN/CUE. Les gravures
ont leur propre dossier `output/burn-*/`. Les échecs et annulations conservent
les fichiers partiels ; il n'y a pas de nettoyage automatique. `job.json` est
écrit à la fin, avec un journal limité aux 600 dernières lignes.

Pour un DVD ISO, prévoyez environ **trois fois la taille du PCM**, plus une
marge : intermédiaires, arborescence et ISO coexistent. Le moteur contrôle
l'espace disponible, mais d'autres programmes peuvent encore le consommer.
Ne mélangez pas les sorties et les sources. Supprimez manuellement les anciens
travaux uniquement lorsqu'ils ne sont plus utilisés.

GTK et web ont des états de travail indépendants. Un même dossier de sortie
empêche les travaux d'authoring/gravure simultanés entre instances, mais pas
l'activité d'un autre logiciel de gravure. Après redémarrage, les anciens
fichiers restent accessibles sur disque ; les travaux ne reprennent pas et
les anciens liens de téléchargement web disparaissent.

L'application ne modifie pas les sources et limite le serveur à la boucle
locale, mais ce n'est **pas un bac à sable** pour les décodeurs de médias.
Utilisez des outils à jour et des fichiers fiables, ou isolez vous-même le
traitement. Testez un petit disque sur le lecteur cible avant un gros projet.

## Développement Et CI

Le [README anglais](README.en.md) conserve la documentation technique complète,
notamment l'architecture du moteur partagé, les limites de formats et les tests.
Go 1.22 ou plus récent est nécessaire ; la CI utilise Go 1.24. Depuis les sources :

```sh
git clone https://github.com/radomd92/kapila-burner.git
cd kapila-burner
make web                 # produit dvd-audio-studio
make gtk                 # produit dvd-audio-gtk ; en-têtes GTK et compilateur C requis
make test
make test-gtk
make smoke-gtk           # en, plt, fr ; affichage ou Xvfb nécessaire
VCD_INTEGRATION=1 make test
# Facultatif en local, après installation des outils DVD-Audio :
DVDA_INTEGRATION=1 VCD_INTEGRATION=1 go test -race ./...
```

Le nom source `cmd/dvd-audio-gtk` et le nom du binaire local `dvd-audio-gtk`
restent inchangés ; les archives publiques utilisent la marque **Kapila Burner**
et le nom `kapila-burner`. Un [overlay GTK de développement](tools/gtk-dev/README.md)
est documenté pour certains hôtes Linux. `make package` crée les archives sur
le système cible avec ses dépendances de développement GTK.

La CI compile les deux interfaces sur Ubuntu 22.04, Windows Server 2022 et
macOS 15 Intel. Sous Linux, elle exécute les tests avec détection de courses,
vet, les tests GTK, les smoke tests dans les trois langues via Xvfb et la
création réelle de VCD avec `VCD_INTEGRATION=1`. **Les outils DVD-Audio ne sont
pas provisionnés et `DVDA_INTEGRATION` n'est pas activé dans la CI hébergée** :
c'est une vérification locale/manuelle facultative. Aucun test CI ne grave de disque.

La première exécution hébergée a compilé et publié les deux interfaces pour
les trois systèmes. Les contrôles graphiques GTK sont exécutés sous Linux ;
sous Windows et macOS, la vérification se limite au lancement avec `-h`.
Pour contribuer, créez une branche depuis `main`,
exécutez les vérifications pertinentes et ouvrez une pull request vers `main`.
