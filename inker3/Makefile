run:
	npm start

msi:
	cd BUILD && build_msi.bat

check:
	node -e "var f=['main.js','preload.js','renderer/renderer.js','lib/mount-manager.js','lib/api-client.js','lib/sync-manager.js','lib/settings.js'],p='c:\\Users\\THA0065\\FtR-Project\\ftr-project\\inker3\\';f.forEach(function(fn){try{require('child_process').execSync('node --check '+p+fn,{stdio:'pipe'});console.log('PASS: '+fn)}catch(e){console.log('FAIL: '+fn+' - '+e.message)}})"
