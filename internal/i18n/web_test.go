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
	script := strings.Split(strings.Split(string(html), "<script>")[1], "</script>")[0]
	check := `
const vm = require('node:vm');
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
});`
	cmd := exec.Command(node, "-e", check)
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("web syntax/catalog: %v\n%s", err, output)
	}
}
