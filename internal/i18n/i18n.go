// Package i18n contains UI-independent English, Plateau Malagasy and French messages.
package i18n

import (
	"context"
	"fmt"
	"strings"
)

type languageKey struct{}

// Normalize accepts language tags and an Accept-Language preference list.
func Normalize(language string) string {
	for _, preference := range strings.Split(strings.ToLower(language), ",") {
		parts := strings.SplitN(strings.TrimSpace(preference), ";", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) == "q=0" {
			continue
		}
		tag := strings.SplitN(parts[0], "-", 2)[0]
		switch tag {
		case "plt", "mg":
			return "plt"
		case "en":
			return "en"
		case "fr":
			return "fr"
		}
	}
	return "en"
}

func WithLanguage(ctx context.Context, language string) context.Context {
	return context.WithValue(ctx, languageKey{}, Normalize(language))
}

func Language(ctx context.Context) string {
	if language, ok := ctx.Value(languageKey{}).(string); ok {
		return language
	}
	return "en"
}

// Text translates application prose. External tool output and OS diagnostics stay verbatim.
func Text(language string, english string, args ...any) string {
	message := english
	var catalog map[string]string
	switch Normalize(language) {
	case "plt":
		catalog = malagasy
	case "fr":
		catalog = french
	}
	if translated, ok := catalog[english]; ok {
		message = translated
	}
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

var malagasy = map[string]string{
	"DVD-Video ISO still exceeds capacity after re-encoding":                                                                              "Mbola mihoatra ny fahafahan'ny kapila ny ISO DVD-Video aorian'ny famadihana indray",
	"Physical VCD burning is supported only on Linux; use external burning software to burn the authored CUE/BIN image on this platform.": "Amin'ny Linux ihany no tohanana ny fandoroana VCD amin'ny kapila; ampiasao rindranasa fandoroana ivelany handoroana ny sarin-kapila CUE/BIN amin'ity rafitra ity.",
	"Burn cancelled; the disc may be unusable. Image files were retained.":                                                                "Nofoanana ny fanoratana; mety tsy ho azo ampiasaina ny kapila. Notazonina ny sarin-kapila.",
	"Not enough free workspace for MPEG tracks and VCD image":                                                                             "Tsy ampy ny toerana malalaka hiasana ho an'ny raki-tsary MPEG sy ny sarin-kapila VCD",
	"Format must be dvd-audio, dvd-video or vcd":                                                                                          "Tsy maintsy dvd-audio, dvd-video na vcd ny endrika",
	"Video standard is only available for DVD-Video and VCD":                                                                              "Ho an'ny DVD-Video sy VCD ihany ny fenitry ny horonantsary",
	"DVD-Video capacity must be dvd5 or dvd9":                                                                                             "Tsy maintsy dvd5 na dvd9 ny fahafahan'ny DVD-Video",
	"DVD-Video standard must be pal or ntsc":                                                                                              "Tsy maintsy pal na ntsc ny fenitry ny DVD-Video",
	"DVD-Video requires one group with 1-99 video clips":                                                                                  "Mila vondrona iray misy horonantsary 1 hatramin'ny 99 ny DVD-Video",
	"DVD-Video requires a video stream: %s":                                                                                               "Mila loharano misy horonantsary ny DVD-Video: %s",
	"DVD-Video cannot fit at the minimum supported quality":                                                                               "Tsy antonona ny DVD-Video na dia amin'ny kalitao ambany indrindra tohanana aza",
	"Not enough free workspace for MPEG tracks, VIDEO_TS and the ISO":                                                                     "Tsy ampy ny toerana ho an'ny rakitra MPEG, VIDEO_TS ary ny ISO",
	"Requires -dvd-video support":                                                                                                         "Mila fanohanana ny -dvd-video",
	"Requires dvdauthor with XML authoring support":                                                                                       "Mila dvdauthor manohana ny famoronana kapila amin'ny XML",
	"DVD-Video requires FFmpeg with MPEG-2 video and AC-3 encoders":                                                                       "Mila FFmpeg afaka mamadika ho horonantsary MPEG-2 sy feo AC-3 ny DVD-Video",
	"DVD-Video attempt %d: %dx%d at %.2f Mb/s (two passes)":                                                                               "Andrana DVD-Video %d: %dx%d amin'ny %.2f Mb/s (dingana roa)",
	"DVD-Video ISO exceeded capacity; retrying with a lower bitrate":                                                                      "Mihoatra ny fahafahan'ny kapila ny ISO DVD-Video; averina amin'ny taha ambany kokoa",
	"DVD-Video ISO is empty or invalid":                                                                                                   "Foana na tsy manankery ny ISO DVD-Video",
	"VCD video standard must be pal or ntsc":                                                                                              "Tsy maintsy pal na ntsc ny fenitry ny VCD",
	"VCD capacity must be cd74 or cd80":                                                                                                   "Tsy maintsy cd74 na cd80 ny tanjaky ny VCD",
	"VCD requires one group with 1-98 sequence tracks":                                                                                    "Mila vondrona iray misy raki-tsary 1-98 ny VCD",
	"VCD exceeds selected CD capacity":                                                                                                    "Mihoatra ny tanjaky ny CD voafidy ny VCD",
	"Creating VCD image only; audio-only tracks use a static black background. No disc will be burned automatically.":                     "Mamorona sarin-kapila VCD ihany; sary mainty tsy mihetsika no ampiasaina amin'ny raki-peo tsy misy sary. Tsy misy kapila hosoratana ho azy.",
	"VCD image is missing, invalid or modified":                                                                                           "Tsy hita, tsy manankery na novaina ny sarin-kapila VCD",
	"Explicit confirmation is required even for a simulated burn; cancellation may ruin the disc":                                         "Mila fanamafisana mazava na dia fitsapana fanoratana aza; mety hanimba ny kapila ny fanafoanana",
	"Burn speed must be between 1 and 52":                                                                                                 "Tsy maintsy eo anelanelan'ny 1 sy 52 ny hafainganam-panoratra",
	"Burn source must be a completed VCD authored by this engine":                                                                         "Tsy maintsy VCD vita noforonin'ity motera ity ny loharano hosoratana",
	"Select a real Linux optical recorder; files and device aliases are not allowed":                                                      "Misafidiana mpanoratra kapila optika Linux tena izy; tsy azo ampiasaina ny rakitra sy ny solon'anaran'ny fitaovana",
	"Optical recorder is busy or its lock is unsafe":                                                                                      "Mbola ampiasaina ny mpanoratra kapila na tsy azo antoka ny hidiny",
	"Burn cancellation may ruin the disc. Image files will be retained.":                                                                  "Mety hanimba ny kapila ny fanafoanana ny fanoratana. Hotazonina ny sarin-kapila.",
	"Tool check failed: %s":                                                                                                               "Tsy nahomby ny fizahana fitaovana: %s",
	"Invalid project JSON: %s":                                                                                                            "Tsy manankery ny JSON tetikasa: %s",
	"Not found; see README.md":                                                                                                            "Tsy hita; jereo ny README.md",
	"Requires real -dvd-audio support; genisoimage is not sufficient":                                                                     "Mila fanohanana tena izy ny -dvd-audio; tsy ampy ny genisoimage",
	"Ready": "Azo ampiasaina",
	"Source must be relative to the audio library: %q": "Tsy maintsy lalana mifandraika amin'ny tahirim-peo ny loharano: %q",
	"Source is outside the audio library: %s":          "Any ivelan'ny tahirim-peo ny loharano: %s",
	"Not a regular file: %s":                           "Tsy rakitra tsotra: %s",
	"ffprobe is unavailable":                           "Tsy azo ampiasaina ny ffprobe",
	"The file contains no audio":                       "Tsy misy feo ao amin'ny rakitra",
	"Unknown or invalid audio duration or format":      "Tsy fantatra na tsy manankery ny faharetana na ny endriky ny feo",
	"Project name must contain 1-200 bytes":            "Tsy maintsy misy 1-200 oktety ny anaran'ny tetikasa",
	"Choose 16-bit or 24-bit PCM":                      "Misafidiana PCM 16-bit na 24-bit",
	"Only mono or stereo PCM is supported":             "PCM mono na stereo ihany no tohanan'ity fitaovana ity",
	"Unsupported DVD-Audio sample rate":                "Tsy tohanana io tahan'ny fakana santionany DVD-Audio io",
	"Capacity must be dvd5 or dvd9":                    "Tsy maintsy dvd5 na dvd9 ny tanjaka",
	"Use 1-9 groups":                                   "Ampiasao ny vondrona 1-9",
	"A group may contain at most 99 tracks":            "Raki-peo 99 raha be indrindra no azon'ny vondrona iray ananana",
	"%s exceeds the 4 GiB WAV limit; split the track or choose a smaller PCM profile": "Mihoatra ny fetra 4 GiB ho an'ny WAV ny %s; zarao ity raki-peo ity na misafidiana firafitra PCM kely kokoa",
	"Add at least one track":                     "Manampia raki-peo iray farafahakeliny",
	"Job cancelled; partial output was retained": "Nofoanana ny asa; notazonina ny vokatra efa vita ampahany",
	"Preparing %.2f hours of audio; estimated disc size %.2f GB. Source files will not be modified.": "Manomana feo maharitra %.2f ora; tombanana ho %.2f GB ny kapila. Tsy hovaina ny rakitra loharano.",
	"Group %d / track %d: %s":                                "Vondrona %d / raki-peo %d: %s",
	"Creating DVD-Audio navigation and AOB files...":         "Mamorona ny fitetezana DVD-Audio sy ny rakitra AOB...",
	"Authoring did not create a valid %s file":               "Tsy namorona rakitra %s manankery ny famoronana kapila",
	"Authoring produced no audio objects":                    "Tsy namokatra singam-peo ny famoronana kapila",
	"Authored disc structure exceeds selected disc capacity": "Mihoatra ny tanjaky ny kapila voafidy ny firafitry ny kapila noforonina",
	"Final ISO is empty or exceeds selected disc capacity":   "Foana ny ISO farany na mihoatra ny tanjaky ny kapila voafidy",
	"Completed: ": "Vita: ",
	"Temporary PCM files were retained for inspection. Burn the ISO as a disc image, not as a file on a data disc.": "Notazonina ny rakitra PCM vonjimaika mba ho azo zahana. Soraty amin'ny kapila ho sarin-kapila ny ISO, fa tsy ho rakitra ao anaty kapila angona.",
	"Decoded PCM exceeds disc capacity":                         "Mihoatra ny tanjaky ny kapila ny PCM voavaha",
	"%s is unavailable or incompatible; check tool status":      "Tsy azo ampiasaina na tsy mifanaraka amin'ny ilaina ny %s; jereo ny satan'ny fitaovana ilaina",
	"Not enough free workspace for PCM, disc structure and ISO": "Tsy ampy ny toerana malalaka hiasana ho an'ny PCM, ny firafitry ny kapila ary ny ISO",
	"Another authoring job is running":                          "Misy asa famoronana kapila hafa mandeha",
	"No job is running":                                         "Tsy misy asa mandeha",
	"Engine is closed":                                          "Efa nakatona ny motera",
	"Tool check failed":                                         "Tsy nahomby ny fizahana fitaovana",
	"Cannot read source: %s":                                    "Tsy azo vakina ny loharano: %s",
	"ffprobe failed: %s":                                        "Tsy nahomby ny ffprobe: %s",
	"Expected a single project JSON object":                     "Zavatra JSON iray ho an'ny tetikasa no andrasana",
	"Use the exact loopback URL shown by the server":            "Ampiasao ilay URL loopback nasehon'ny lohamilina, tsy misy fanovana",
	"Cross-origin requests are not allowed":                     "Tsy azo atao ny fangatahana avy amin'ny fiaviana hafa",
	"Cross-site requests are not allowed":                       "Tsy azo atao ny fangatahana avy amin'ny tranonkala hafa",
	"Command failed: %s":                                        "Tsy nahomby ny baiko: %s",
	"Cannot persist job status: %s":                             "Tsy voatahiry ny satan'ny asa: %s",
}
