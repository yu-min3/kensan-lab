// CI runs yarn tsc before this focused runtime gate. No native TS loader needed.
const {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  rmSync,
} = require('node:fs');
const { resolve } = require('node:path');
const { spawnSync } = require('node:child_process');
const ts = require('typescript');

const backstage = resolve(__dirname, '..');
const repository = resolve(backstage, '..');
const cache = resolve(backstage, '.cache');
mkdirSync(cache, { recursive: true });
const build = mkdtempSync(resolve(cache, 'sense-canary-test-'));

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: repository,
    env: {
      ...process.env,
      SENSE_CANARY_TEST_BUILD: build,
      PYTHONDONTWRITEBYTECODE: '1',
    },
    stdio: 'inherit',
  });
  if (result.error) throw result.error;
  if (result.status !== 0)
    throw new Error(
      `${command} tests failed (${result.status ?? result.signal})`,
    );
}

try {
  for (const name of ['senseCanary', 'senseCanaryPolicy']) {
    const file = resolve(backstage, `packages/backend/src/modules/${name}.ts`);
    const compiled = ts.transpileModule(readFileSync(file, 'utf8'), {
      fileName: file,
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
        esModuleInterop: true,
      },
    });
    writeFileSync(resolve(build, `${name}.js`), compiled.outputText);
  }
  const policyTest = resolve(__dirname, 'sense-canary-policy.node-test.ts');
  const compiledTest = ts.transpileModule(readFileSync(policyTest, 'utf8'), {
    fileName: policyTest,
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
      esModuleInterop: true,
    },
    transformers: {
      before: [
        context => source =>
          ts.visitNode(source, function visit(node) {
            if (
              ts.isImportDeclaration(node) &&
              node.moduleSpecifier.text ===
                '../packages/backend/src/modules/senseCanaryPolicy.ts'
            ) {
              return context.factory.updateImportDeclaration(
                node,
                node.modifiers,
                node.importClause,
                context.factory.createStringLiteral('./senseCanaryPolicy.js'),
                node.attributes,
              );
            }
            return ts.visitEachChild(node, visit, context);
          }),
      ],
    },
  });
  writeFileSync(resolve(build, 'policy.test.cjs'), compiledTest.outputText);
  run(process.execPath, [
    '--test',
    resolve(__dirname, 'sense-canary-api.test.cjs'),
    resolve(build, 'policy.test.cjs'),
  ]);
  run('python3', [
    resolve(repository, 'scripts/tests/test-sense-canary-generate.py'),
  ]);
} finally {
  rmSync(build, { recursive: true, force: true });
}
