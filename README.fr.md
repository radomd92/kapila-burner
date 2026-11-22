# kapila-maker

[Malagasy (Plateau)](README.md) | [English](README.en.md) | **Français**

[![CI](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml/badge.svg)](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

L'application s'appelle `kapila-maker` ; son dépôt GitHub garde le nom `kapila-burner`.

Un atelier local pour créer des disques **DVD-Audio LPCM non compressé** et des
images **Video CD 2.0**, avec une interface native GTK ou une interface web locale.
Les deux interfaces utilisent le même moteur, sans service en ligne.
La création d'une image précède toute gravure : rien n'est gravé automatiquement.

## Télécharger

Préversion continue (`continuous`), actualisée après une CI réussie.

**Transition en cours : la vérification des nouveaux paquets est en attente.**
La version actuellement publiée reste l'ancienne `kapila-burner`, avec trois fichiers
et des outils d'authoring externes. La première CI native a échoué et le nouveau
pipeline des utilitaires est encore en test. Les nouveaux fichiers
`kapila-maker` restent **en attente jusqu'à la réussite de la CI et à la confirmation
de leur publication complète**. Les nouveaux liens peuvent renvoyer une erreur 404
d'ici là. Les descriptions des nouveaux paquets ci-dessous ne prouvent pas leur
validation ni leur disponibilité.

Installateurs (publication en attente) :
[Windows EXE](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64-setup.exe)
| [macOS Intel DMG](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64.dmg)

Alternatives portables (nouveaux noms, publication en attente) :

| Plateforme | Archive |
| --- | --- |
| Linux x86-64 | [kapila-maker-linux-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-linux-amd64.tar.gz) |
| Windows x86-64 | [kapila-maker-windows-amd64.zip](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64.zip) |
| macOS Intel x86-64 | [kapila-maker-macos-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64.tar.gz) |

[SHA256SUMS](https://github.com/radomd92/kapila-burner/releases/download/continuous/SHA256SUMS)
| [Page de la préversion](https://github.com/radomd92/kapila-burner/releases/tag/continuous)
| [Workflow CI et artefacts](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

Chaque exécution réussie suivante actualise cette préversion. Les pull requests
produisent des artefacts Actions conservés **14 jours**, sans publication de
release. Les tags `v*` publient des versions distinctes.

Chaque archive contient **`kapila-maker`** (GTK natif) et
**`kapila-maker-web`** (web portable), avec l'extension `.exe` sous Windows.
Extrayez l'archive avant utilisation. Le programme web ne nécessite pas GTK.
Le nouveau tar.gz macOS contient aussi **`kapila-maker.app`**, ainsi que les
exécutables natif/web de commodité. Les nouveaux paquets Windows/macOS incluent
GTK, toutes les bibliothèques non système et données nécessaires et les cinq
outils de création d'images, pour les deux interfaces.

| Plateforme | Exigences de l'interface GTK |
| --- | --- |
| Linux | Ubuntu 22.04 ou plus récent, ou distribution compatible, avec GTK 3 système (`libgtk-3-0` sous Ubuntu) et une session graphique. |
| Windows | Installez l'EXE ou conservez **tout le dossier ZIP extrait** et lancez `kapila-maker.exe` / `launch-gtk.cmd`, pas un `.exe` déplacé seul. Les DLL GTK, bibliothèques et données sont incluses. |
| macOS | Intel macOS 15 ou plus récent. Ouvrez le DMG et glissez l'application autonome **`kapila-maker.app`** dans Applications, ou utilisez le tar.gz. GTK, bibliothèques et données sont incluses ; aucun Homebrew requis. |

Aucun exécutable ARM n'est publié. Sous Linux, les outils sont inclus ; GTK 3 et
les bibliothèques VCD restent fournis par le système. La gravure physique de VCD est réservée
à **Linux**, avec `cdrdao` facultatif et confirmation explicite. La gravure n'est
pas prise en charge sous Windows/macOS : utilisez un logiciel externe pour graver
les images. Ce logiciel n'est **pas requis pour créer les images** ; rien n'est
gravé automatiquement.

L'installateur Windows s'installe par utilisateur dans
`%LOCALAPPDATA%\Programs\kapila-maker`, sans droits administrateur, avec raccourcis
et désinstalleur. Il est **non signé** : Windows/SmartScreen peut afficher un
avertissement ; aucun certificat de signature de code n'est revendiqué.
L'application macOS porte une **signature ad hoc, sans signature Developer ID ni
notarisation**. Respectez les règles normales de Gatekeeper et de votre organisation,
sans désactiver les protections globales. Les utilisateurs des nouveaux paquets
Windows/macOS n'ont besoin ni de Homebrew, MSYS2, Go, Python, shell, compilateur,
installation d'outils séparée, ni de configuration du `PATH`. Les outils de
compilation CI ne sont pas des prérequis utilisateur.

Après publication, `SHA256SUMS` couvrira **huit fichiers** : trois archives
portables, deux installateurs (EXE/DMG) et trois archives de sources correspondantes.

Téléchargez `SHA256SUMS` pour vérifier l'intégrité des archives. Sous Linux :

```sh
sha256sum --check --ignore-missing SHA256SUMS
```

Sous macOS, comparez `shasum -a 256 kapila-maker-macos-amd64.tar.gz` à la valeur
publiée. Sous PowerShell, utilisez
`Get-FileHash .\kapila-maker-windows-amd64.zip -Algorithm SHA256`.
Ces sommes de contrôle **ne sont pas des signatures cryptographiques**.

## Démarrer

Sous Windows, utilisez le raccourci installé ; sous macOS, ouvrez
`kapila-maker.app` dans Applications. Aucun terminal n'est nécessaire pour ces
lancements graphiques. En option, depuis le dossier extrait sous Linux ou macOS :

```sh
./kapila-maker -root /path/to/media
# Démarrer directement en français :
./kapila-maker -root /path/to/media -lang fr
# Ou en anglais :
./kapila-maker -root /path/to/media -lang en
# Autre interface : navigateur local
./kapila-maker-web -root /path/to/media
```

Avec le ZIP portable Windows, ouvrez `kapila-maker.exe` / `launch-gtk.cmd`, ou
lancez depuis PowerShell :

```powershell
.\launch-gtk.cmd -root C:\Music -lang fr
# Interface web :
.\kapila-maker-web.exe -root C:\Music
```

Les nouveaux paquets trouvent les outils relativement à l'exécutable,
depuis **n'importe quel répertoire de travail**. Par défaut, les médias viennent du
dossier Music de l'utilisateur et les sorties vont dans `kapila-maker/output`
sous son cache utilisateur. `-root` et `-output` remplacent ces valeurs.
Sous Linux, extrayez l'archive entière avec `tools/` et `BUILD-UTILITIES.txt`.
Les compilations depuis les sources conservent les anciens défauts relatifs au
répertoire courant : médias dans `..`, sorties dans `./output` et recherche locale
des outils. Remplacez la racine des exemples par votre dossier de médias.

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

Dans GTK, choisir un dossier ne déclenche aucune analyse. Ouvrez un projet puis
cliquez sur **Analyser le dossier** pour rechercher les médias dans le dossier
sélectionné, à l'intérieur de `-root`.

Le web conserve automatiquement le projet dans ce navigateur, mais son stockage
peut être effacé : exportez les projets importants. GTK Open/Save et web
Import/Export échangent le même JSON. Ce manifeste n'est **pas une sauvegarde
des médias** : les sources doivent rester aux mêmes chemins relatifs à la racine.

## Outils Externes

Les **nouveaux paquets, encore en attente de validation/publication**,
incluent les cinq outils de création d'images compilés depuis les sources :
FFmpeg/FFprobe **9.0.2**, VCDImager **2.0.1**, le coeur `dvda-author` et un véritable
`mkisofs` compatible DVD-Audio. Les versions/commits épinglés, URL et empreintes
figurent dans le [manifeste des sources](scripts/utility-sources.tsv).
Sous **Linux**, consultez le [guide des dépendances](README.en.md#dependencies)
pour les bibliothèques système GTK et VCD. `cdrdao` reste facultatif pour la
gravure. Dans GTK, **Outils externes** affiche l'état, les chemins et les diagnostics.

| Fonction | Outils nécessaires |
| --- | --- |
| Analyse et conversion | FFprobe et FFmpeg |
| DVD-Audio | `dvda-author` ; un véritable `mkisofs` prenant en charge `-dvd-audio` pour créer l'ISO. Un substitut `genisoimage` sans cette option ne convient pas. |
| Video CD | VCDImager (`vcdimager`) ; les outils DVD ne sont pas nécessaires. |
| Gravure VCD GTK, Linux uniquement | `cdrdao`, un graveur CD et un CD-R/CD-RW vierge. |

Vous pouvez imposer des chemins absolus avec `FFMPEG`, `FFPROBE`, `DVDA_AUTHOR`,
`MKISOFS`, `VCDIMAGER` et `CDRDAO`. Les nouveaux paquets trouvent leurs
outils inclus relativement à l'exécutable, sans configuration du `PATH`.
Pour les compilations Linux depuis les sources, seuls `dvda-author` et
`mkisofs` sont recherchés dans `./tools/bin/` avant le `PATH` ; les autres
utilisent le `PATH`. Redémarrez l'interface après un changement d'outils.
Consultez le [guide détaillé](README.en.md#dependencies) pour les outils locaux
facultatifs et leurs limites de portabilité.

### Sources Et Licences

Les archives correspondantes
[`kapila-maker-linux-amd64-sources.tar.gz`](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-linux-amd64-sources.tar.gz),
[`kapila-maker-windows-amd64-sources.tar.gz`](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-windows-amd64-sources.tar.gz)
et
[`kapila-maker-macos-amd64-sources.tar.gz`](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-maker-macos-amd64-sources.tar.gz)
seront publiées avec les nouveaux paquets ; elles sont aussi en attente.
**Elles ne sont pas nécessaires pour exécuter l'application.** Elles contiennent
les archives sources exactes des formules du runtime macOS, toutes les sources
des paquets MSYS2 correspondants, les sources originales des utilitaires, les
correctifs, scripts et licences. Conservez les notices ; chaque composant garde
sa licence. Le [guide anglais](README.en.md#corresponding-sources) décrit les
manifestes et la reconstruction facultative des utilitaires hors ligne.

## Créer Un DVD-Audio

1. Choisissez **DVD Audio**, puis cliquez sur **Analyser le dossier** dans GTK.
   Attendez la fin de l'analyse avant de sélectionner les pistes.
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
Ces chemins sont relatifs à la racine de sortie choisie : par défaut, cache
utilisateur `kapila-maker/output` pour les nouveaux paquets,
`./output` pour les compilations depuis les sources.

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
make web                 # produit kapila-maker-web
make gtk                 # produit kapila-maker ; en-têtes GTK et compilateur C requis
make test
make test-gtk
make smoke-gtk           # en, plt, fr ; affichage ou Xvfb nécessaire
VCD_INTEGRATION=1 make test
# Facultatif en local, après installation des outils DVD-Audio :
DVDA_INTEGRATION=1 VCD_INTEGRATION=1 go test -race ./...
```

Le code GTK se trouve dans `cmd/kapila-maker` ; les binaires locaux et publics
s'appellent `kapila-maker` et `kapila-maker-web`.
Un [overlay GTK de développement](tools/gtk-dev/README.md)
est documenté pour certains hôtes Linux. `make package` crée les archives sur
le système cible avec ses dépendances de développement GTK.

La CI compile les deux interfaces sur Ubuntu 22.04, Windows Server 2022 et
macOS 15 Intel. Sous Linux, elle exécute les tests avec détection de courses,
vet, les tests GTK, les smoke tests dans les trois langues via Xvfb et la
création réelle de VCD avec `VCD_INTEGRATION=1`. La nouvelle CI de bundling prévoit
la compilation source des utilitaires Windows/macOS, la création réelle d'images
DVD-Audio et VCD avec un `PATH` nettoyé et le contrôle d'une `.app` déplacée.
La collecte des sources doit échouer si des sources correspondantes manquent.
**Ces nouveaux contrôles sont encore en attente, pas validés.** Aucun test CI
ne grave de disque ni ne vérifie la lecture sur matériel.

Les anciens succès CI ne prouvent pas le bon fonctionnement des nouveaux paquets.
Les contrôles graphiques GTK restent sous Linux ; les nouveaux tests d'images et
de déplacement Windows/macOS vont au-delà de l'ancien lancement avec `-h`, sans
prouver l'interaction graphique. Consultez les logs CI et les fichiers publiés.
Pour contribuer, créez une branche depuis `main`,
exécutez les vérifications pertinentes et ouvrez une pull request vers `main`.
