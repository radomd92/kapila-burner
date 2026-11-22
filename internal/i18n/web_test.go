package i18n

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWebCatalog(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required to check the web catalog")
	}
	html, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<option value="dvd-video">DVD-Video</option>`) {
		t.Fatal("DVD-Video format option is missing")
	}
	script := strings.Split(strings.Split(string(html), "<script>")[1], "</script>")[0]
	check := `
const vm = require('node:vm');
const assert = require('node:assert/strict');
let source = ''; process.stdin.on('data', chunk => source += chunk);
process.stdin.on('end', () => {
  new vm.Script(source);
  const prefix = source.slice(0, source.indexOf('const walker ='));
  for (const saved of [null, '', 'invalid', 'plt', 'en', 'fr']) {
    const context = {localStorage:{getItem:() => saved}, document:{}};
    vm.createContext(context);
    vm.runInContext(prefix + '\n globalThis.result = {language,catalog,french,t};', context);
    const {language,catalog,french,t} = context.result;
    if (language !== (['plt','en','fr'].includes(saved) ? saved : 'plt')) throw Error('Wrong default: ' + saved);
    for (const [mg,en] of catalog) {
      if (!french.has(en)) throw Error('Missing French: ' + en);
      const placeholders = value => JSON.stringify((value.match(/\{\d+\}/g) || []).sort());
      if (placeholders(mg) !== placeholders(en) || placeholders(en) !== placeholders(french.get(en))) throw Error('Placeholder mismatch: ' + en);
    }
    const path = '/tmp/100%.wav';
    if (!t('Vokatra: {0}',path).endsWith(path)) throw Error('Path changed');
    if (t('raw external diagnostic') !== 'raw external diagnostic') throw Error('Diagnostic changed');
  }
  const nodes = new Map();
  const element = id => {
    if (!nodes.has(id)) nodes.set(id,{value:'',closest:() => element(id + '-label'),replaceChildren(...children) { this.children = children; },addEventListener() {}});
    return nodes.get(id);
  };
  const document = {getElementById:element,querySelector:element,createElement:() => ({})};
  const ui = source.slice(source.indexOf('const freshProject ='),source.indexOf('try { const saved = localStorage.getItem(STORAGE_KEY)'));
  vm.runInNewContext(prefix + ui + '\n' + checkDVDVideo.toString() + '\ncheckDVDVideo();', {document,assert,localStorage:{getItem:() => 'en'},confirm:() => true});
});`
	check += `
function checkDVDVideo() {
  const clip = 'concert.mp4', audio = 'concert.wav';
  const input = {name:'Concert',format:'dvd-video',capacity:'dvd9',videoStandard:'ntsc',iso:false,groups:[{name:'Clips',tracks:[clip,clip]}]};
  const exported = validateProject(input);
  assert.deepEqual(validateProject(JSON.parse(JSON.stringify(exported))),exported);
  assert.equal(exported.videoStandard,'ntsc');
  assert.equal(exported.iso,true);
  assert.equal(exported.sampleRate,0);
  assert.equal(validateProject({...input,videoStandard:''}).videoStandard,'pal');
  assert.throws(() => validateProject({...input,videoStandard:'secam'}));
  assert.throws(() => validateProject({...input,capacity:'cd80'}));
  assert.throws(() => validateProject({...input,groups:[...input.groups,...input.groups]}));
  const withClips = tracks => ({...input,groups:[{name:'Clips',tracks}]});
  assert.throws(() => validateProject(withClips([])));
  assert.equal(validateProject(withClips([]),{draft:true}).groups[0].tracks.length,0);
  assert.throws(() => validateProject(withClips(['../clip.mp4'])));
  assert.equal(validateProject(withClips(Array(99).fill(clip))).groups[0].tracks.length,99);
  assert.throws(() => validateProject(withClips(Array(100).fill(clip))));

  files = [{path:audio},{path:clip,video:true},{path:'bad.mp4',video:true,error:'bad source'}];
  fileMap = new Map(files.map(file => [file.path,file]));
  libraryReady = true;
  assert.throws(() => validateProject(withClips([audio])));
  project.groups.push({name:'Second',tracks:[clip]});
  selected.add(audio);
  selected.add(clip);
  selected.add('bad.mp4');
  hydrate = renderFormat; persist = () => {}; renderLibrary = () => {};
  $('project-format').onchange({target:{value:'dvd-video'}});
  assert.equal(project.format,'dvd-video');
  assert.equal(project.groups.length,1);
  assert.deepEqual(project.groups[0].tracks,[clip]);
  assert.deepEqual([...selected],[clip]);
  assert.deepEqual(visibleFiles().map(file => file.path),[clip,'bad.mp4']);
  assert.equal($('video-standard-label').hidden,false);
  assert.deepEqual(Array.from($('capacity').children,option => option.value),['dvd5','dvd9']);
  for (const id of ['sample-rate','bits','channels','iso']) assert.equal($(id).disabled,true);
  assert.equal(videoEncodingEstimate({videoBitrate:3500000,videoWidth:352}),'3.5 Mb/s / 352 px wide');
  assert.equal(videoEncodingEstimate({videoBitrate:8000000,videoWidth:720}),'8 Mb/s / 720 px wide');
  assert.equal(videoEncodingEstimate({}),'--');
}
`
	cmd := exec.Command(node, "-e", check)
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("web syntax/catalog: %v\n%s", err, output)
	}
}
