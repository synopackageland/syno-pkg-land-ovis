import assert from 'node:assert/strict';
import test from 'node:test';
import {renderSharedFolderWizardFiles} from '@synopackageland/cli/dist/standalone/shared-folder.js';
const folder={mode:'select',key:'data',permission:'rw',picker:'folder'};

test('the published wizard keeps the old-DSM fallback and adds the native chooser dynamically',()=>{
 const files=renderSharedFolderWizardFiles('Ovis',folder);
 const all=Object.values(files).join('\n');
 assert.match(all,/data_subpath/);assert.match(all,/data/);
 assert.ok(Object.keys(files).some(k=>k.endsWith('.sh')));
});
