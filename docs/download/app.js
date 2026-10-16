/* No analytics, fingerprinting or requests to third-party detection services. */
'use strict';
const release = '0.4.10';
const base = `https://github.com/SeredaLabs/mcp-connect/releases/download/v${release}/`;
const $ = id => document.getElementById(id);
let uk = /^uk\b/i.test(navigator.language);
const copy = {
 en: {title:'Connect your AI apps.',subtitle:'One compact utility. Your credentials stay in your system vault.',downloadTitle:'Download for your computer',osLabel:'Operating system',archLabel:'Processor',helpTitle:'Which processor do I have?',helpText:'macOS: Apple menu → About This Mac → Chip / Processor. Windows: Settings → System → About → System type. Linux: run uname -m; x86_64 means Intel/AMD, aarch64 means ARM64.',notice:'Preview: macOS and Windows installers are not publisher-signed. Your OS may ask for approval. Windows ARM64 graphical support is experimental.',docs:'Documentation',all:'All packages & checksums',detected:'Detected',unknown:'Choose your desktop operating system.',mobile:'This is a desktop utility. Choose the computer where you will install it.',archHint:'Your browser does not reliably identify the processor. Please select it once.',choose:'Choose your computer above',download:'Download installer',portable:'Portable archive',darwin:'macOS 13 or later · installs in Applications',windows:'64-bit Windows · installs for your user, without administrator rights',linux:'Ubuntu 24.04 or compatible · .deb installer. For other distributions use the portable archive; GUI libraries and Secret Service are required.'},
 uk: {title:'Підключайте свої AI-програми.',subtitle:'Одна компактна утиліта. Облікові дані залишаються в захищеному сховищі ОС.',downloadTitle:'Завантажити для вашого комп’ютера',osLabel:'Операційна система',archLabel:'Процесор',helpTitle:'Як дізнатися свій процесор?',helpText:'macOS: меню Apple → Про цей Mac → Чип / Процесор. Windows: Налаштування → Система → Про систему → Тип системи. Linux: команда uname -m; x86_64 — Intel/AMD, aarch64 — ARM64.',notice:'Preview: установники macOS і Windows не мають підпису видавця. ОС може попросити дозвіл. Графічна підтримка Windows ARM64 експериментальна.',docs:'Документація',all:'Усі пакунки й контрольні суми',detected:'Визначено',unknown:'Виберіть операційну систему комп’ютера.',mobile:'Це настільна утиліта. Виберіть комп’ютер, на якому її встановите.',archHint:'Браузер не дозволяє надійно визначити процесор. Виберіть його один раз.',choose:'Виберіть свій комп’ютер вище',download:'Завантажити установник',portable:'Портативна версія',darwin:'macOS 13 або новіша · встановлення в Програми',windows:'64-бітна Windows · встановлення для поточного користувача без прав адміністратора',linux:'Ubuntu 24.04 або сумісна · установник .deb. Для інших дистрибутивів — портативна версія; потрібні графічні бібліотеки та Secret Service.'}
};
function detectPlatform(ua, platform, mobile) {
 if (mobile || /Android|iPhone|iPad|iPod/i.test(ua)) return '';
 if (/Windows|Win32|Win64/i.test(platform+' '+ua)) return 'windows';
 if (/macOS|MacIntel|Macintosh/i.test(platform+' '+ua)) return 'darwin';
 if (/Linux/i.test(platform+' '+ua)) return 'linux';
 return '';
}
function detectArch(ua, hints) {
 if (hints?.bitness === '64' && hints.architecture === 'arm') return 'arm64';
 if (hints?.bitness === '64' && hints.architecture === 'x86') return 'amd64';
 if (/aarch64|arm64/i.test(ua)) return 'arm64';
 // macOS UA often says Intel even on Apple Silicon. Never infer its chip from it.
 if (!/Macintosh|MacIntel/i.test(ua) && /x86_64|Win64; x64|WOW64|amd64/i.test(ua)) return 'amd64';
 return '';
}
function render() {
 const t=copy[uk?'uk':'en'];
 document.documentElement.lang=uk?'uk':'en';
 for(const id of ['title','subtitle','downloadTitle','osLabel','archLabel','helpTitle','helpText','notice','docs','all']) $(id).textContent=t[id];
 $('language').textContent=uk?'English':'Українська';
 $('os').options[0].textContent=uk?'Виберіть…':'Choose…'; $('arch').options[0].textContent=uk?'Виберіть…':'Choose…';
 $('docs').href=uk?'https://github.com/SeredaLabs/mcp-connect/blob/main/docs/INSTALL.uk.md':'https://github.com/SeredaLabs/mcp-connect';
 const os=$('os').value, arch=$('arch').value;
 $('detected').textContent=os?`${t.detected}: ${$('os').selectedOptions[0].textContent}`:(navigator.userAgentData?.mobile || /Android|iPhone|iPad/i.test(navigator.userAgent)?t.mobile:t.unknown);
 $('archHint').textContent=os&&!arch?t.archHint:'';
 $('requirements').textContent=t[os]||'';
 const ready=!!os&&!!arch;
 const suffix=os==='darwin'?'installer.pkg':os==='windows'?'setup.exe':null;
 const installer=os==='linux'?`mcp-connect-linux-${arch}.deb`:`mcp-connect-${os}-${arch}-${suffix}`;
 const portable=`mcp-connect-${os}-${arch}-portable.${os==='linux'?'tar.gz':'zip'}`;
 for(const [id,file] of [['installer',installer],['portable',portable]]) {
  const link=$(id); link.classList.toggle('disabled',!ready); link.setAttribute('aria-disabled',String(!ready));
  if(ready) link.href=base+file; else link.removeAttribute('href');
 }
 $('installer').textContent=ready?`${t.download} · ${$('os').selectedOptions[0].textContent} ${arch==='arm64'?'ARM64':'Intel / AMD'}`:t.choose;
 $('portable').textContent=t.portable;
}
async function init() {
 const ua=navigator.userAgent; const data=navigator.userAgentData;
 $('os').value=detectPlatform(ua,data?.platform||navigator.platform,data?.mobile || (navigator.platform==='MacIntel'&&navigator.maxTouchPoints>1));
 $('arch').value=detectArch(ua,null); render();
 try {
  const hints=await data?.getHighEntropyValues(['architecture','bitness']);
  if (!$('arch').value && hints) {$('arch').value=detectArch(ua,hints); render();}
 } catch { /* Manual selection remains available when hints are denied. */ }
}
$('os').addEventListener('change',()=>{$('arch').value='';render();});
$('arch').addEventListener('change',render);
$('language').addEventListener('click',()=>{uk=!uk;render();});
init();
