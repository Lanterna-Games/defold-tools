"use strict";

const vscode = require("vscode");
const { spawn, execFile } = require("child_process");
const fs = require("fs");
const path = require("path");
const { FileView } = require("./fileView");

const PROJECT_FILE = "game.project";
const LIBRARY_SETTING = "workspace.library";
const RUNTIME_SETTING = "runtime.version";
const RUNTIME_OF_DEFOLD = "Lua 5.1";
const SCRIPTS = "**/*.{script,gui_script,render_script,lua}";
const CHARACTERS_THAT_START_A_NAME = ['"', "'", "#", "/", ":", "."];
const CHARACTERS_THAT_START_A_SETTING = ["[", "="];
const CHARACTERS_THAT_START_A_VALUE = ['"', "/"];
const SETTINGS = "game.project";
const DIRECTORIES_NOT_SEARCHED = "{build,.internal,.git}/**";
const MOST_FILES_OFFERED = 2000;
const DATA =
  "**/*.{go,collection,gui,atlas,tilemap,tilesource,sprite,label,collisionobject,convexshape,collectionfactory,collectionproxy,factory,particlefx,model,mesh,sound,font,material,render,render_target,camera,spinemodel,spinescene,cubemap,display_profiles,input_binding,gamepads,texture_profiles,animationset,light,compute}";
const COMMANDS_THAT_USE_THE_EDITOR = ["build", "run", "resolve", "clean", "bundle"];
const PLATFORMS_TO_BUNDLE_FOR = [
  { label: "Windows", platform: "x86_64-win32" },
  { label: "Linux", platform: "x86_64-linux" },
  { label: "macOS, Apple silicon", platform: "arm64-macos" },
  { label: "macOS, Intel", platform: "x86_64-macos" },
  { label: "Web", platform: "wasm-web" },
  { label: "Android", platform: "arm64-android" },
  { label: "iOS", platform: "arm64-ios" },
];
const DEBUGGER = "defold";
const DEFOLD_KIT = "astronachos.defold";
const LAUNCH_CONFIGURATION = { type: DEBUGGER, request: "launch", name: "Defold: Build and Debug" };

function wasLeftByDefoldKit(configuration) {
  return configuration?.type === "lua-local" && /defold ?kit/i.test(JSON.stringify(configuration));
}
const FIRST_REQUEST_OF_A_PROVIDER = 10;
const ANNOTATIONS_OF_OTHERS = /astronachos\.defold|defold-vet[\\/]annotations/;
const PAUSE_BEFORE_TELLING_OF_FILES = 300;
const DIRECTORIES_THE_BUILD_WRITES = /[\\/](build|\.internal|\.git)[\\/]/;
const SEVERITIES = {
  1: vscode.DiagnosticSeverity.Error,
  2: vscode.DiagnosticSeverity.Warning,
  3: vscode.DiagnosticSeverity.Information,
  4: vscode.DiagnosticSeverity.Hint,
};

class Server {
  constructor(folder, diagnostics, log) {
    this.folder = folder;
    this.diagnostics = diagnostics;
    this.log = log;
    this.received = Buffer.alloc(0);
    this.reported = new Set();
    this.subscriptions = [];
    this.isReady = false;
    this.waiting = new Map();
    this.nextRequest = FIRST_REQUEST_OF_A_PROVIDER;
  }

  ask(method, params) {
    if (!this.isReady) {
      return Promise.resolve(null);
    }
    const id = this.nextRequest++;
    return new Promise((resolve, reject) => {
      this.waiting.set(id, { resolve, reject });
      this.send({ id, method, params });
    });
  }

  askAbout(method, document, position, range, more) {
    return this.ask(method, {
      textDocument: { uri: document.uri.toString() },
      position: position ? { line: position.line, character: position.character } : undefined,
      range: range ? asWritten(range) : undefined,
      ...more,
    });
  }

  play(command, ...files) {
    const settings = vscode.workspace.getConfiguration("defoldTools", this.folder);
    const home = settings.get("defoldHome");
    const usesTheEditor = COMMANDS_THAT_USE_THE_EDITOR.includes(command);
    const asked = [command, ...(home && usesTheEditor ? ["--defold-home", home] : []), ...files];
    const task = new vscode.Task(
      { type: "defold", command },
      this.folder,
      command,
      "Defold",
      new vscode.ProcessExecution(settings.get("server.path"), asked, { cwd: this.folder.uri.fsPath }),
      ["$defold-build", "$defold-run"],
    );
    task.presentationOptions = { clear: true, showReuseMessage: false };
    return vscode.tasks.executeTask(task).then((started) => {
      if (command === "run") {
        this.game = started;
      }
      return started;
    });
  }

  reload(document) {
    const isAScript = vscode.languages.match({ pattern: new vscode.RelativePattern(this.folder, SCRIPTS) }, document) > 0;
    if (!this.game || !isAScript) {
      return;
    }
    const settings = vscode.workspace.getConfiguration("defoldTools", this.folder);
    const home = settings.get("defoldHome");
    const asked = ["reload", ...(home ? ["--defold-home", home] : []), "--project", this.folder.uri.fsPath, document.uri.fsPath];
    const reloading = spawn(settings.get("server.path"), asked, { cwd: this.folder.uri.fsPath });
    reloading.stdout.on("data", (text) => this.log.append(text.toString()));
    reloading.stderr.on("data", (text) => this.log.append(text.toString()));
    reloading.on("error", (error) => this.log.appendLine(`the game was not told to reload: ${error.message}`));
  }

  hasStopped(execution) {
    if (execution === this.game) {
      this.game = undefined;
    }
  }

  askQuietly(...about) {
    return this.askAbout(...about).catch(() => null);
  }

  start() {
    const program = vscode.workspace.getConfiguration("defoldTools", this.folder).get("server.path");
    this.process = spawn(program, [], { cwd: this.folder.uri.fsPath });
    this.process.on("error", (error) => {
      vscode.window.showErrorMessage(`defoldpls could not be started as "${program}": ${error.message}. Set defoldTools.server.path to where it is.`);
    });
    this.process.stderr.on("data", (text) => this.log.append(text.toString()));
    this.process.stdout.on("data", (bytes) => this.read(bytes));
    this.send({ id: 1, method: "initialize", params: { processId: process.pid, rootUri: this.folder.uri.toString(), capabilities: {} } });
  }

  stop() {
    this.subscriptions.forEach((subscription) => subscription.dispose());
    this.subscriptions = [];
    this.waiting.forEach(({ resolve }) => resolve(null));
    this.waiting.clear();
    clearTimeout(this.tellingOfFiles);
    this.reported.forEach((uri) => this.diagnostics.delete(vscode.Uri.parse(uri)));
    this.reported.clear();
    if (this.process && this.process.exitCode === null) {
      this.send({ id: 2, method: "shutdown" });
      this.send({ method: "exit" });
      this.process.stdin.end();
    }
  }

  send(message) {
    if (!this.process || !this.process.stdin.writable) {
      return;
    }
    const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...message }), "utf8");
    this.process.stdin.write(`Content-Length: ${body.length}\r\n\r\n`);
    this.process.stdin.write(body);
  }

  read(bytes) {
    this.received = Buffer.concat([this.received, bytes]);
    for (;;) {
      const endOfHeaders = this.received.indexOf("\r\n\r\n");
      if (endOfHeaders < 0) {
        return;
      }
      const length = /content-length:\s*(\d+)/i.exec(this.received.subarray(0, endOfHeaders).toString("ascii"));
      const start = endOfHeaders + 4;
      const end = start + (length ? Number(length[1]) : 0);
      if (this.received.length < end) {
        return;
      }
      const body = this.received.subarray(start, end).toString("utf8");
      this.received = this.received.subarray(end);
      this.handle(JSON.parse(body));
    }
  }

  handle(message) {
    if (this.waiting.has(message.id) && !message.method) {
      const { resolve, reject } = this.waiting.get(message.id);
      this.waiting.delete(message.id);
      if (message.error) {
        reject(new Error(message.error.message));
      } else {
        resolve(message.result ?? null);
      }
    } else if (message.id === 1 && !message.method) {
      this.becomeReady();
    } else if (message.method === "textDocument/publishDiagnostics") {
      this.show(message.params);
    } else if (message.method === "defold/annotations") {
      this.annotations = message.params;
      this.offerAnnotations();
    } else if (message.method === "window/showMessage") {
      const show = message.params.type === 2 ? vscode.window.showWarningMessage : vscode.window.showErrorMessage;
      show(message.params.message);
    } else if (message.method === "window/logMessage") {
      this.log.appendLine(message.params.message);
    }
  }

  library() {
    return vscode.workspace.getConfiguration("Lua", this.folder).get(LIBRARY_SETTING) ?? [];
  }

  declaresARuntime() {
    const written = vscode.workspace.getConfiguration("Lua", this.folder).inspect(RUNTIME_SETTING);
    return [written?.globalValue, written?.workspaceValue, written?.workspaceFolderValue].some((value) => value !== undefined);
  }

  async offerAnnotations() {
    if (!vscode.extensions.getExtension("sumneko.lua") || this.library().includes(this.annotations.directory)) {
      return;
    }
    const use = "Use them";
    const chosen = await vscode.window.showInformationMessage(
      `The API annotations of Defold ${this.annotations.defold} are ready for the Lua language server. Using them writes Lua.workspace.library${this.declaresARuntime() ? "" : " and Lua.runtime.version"} in the settings of ${this.folder.name}.`,
      use,
      "Not now",
    );
    if (chosen === use) {
      await this.useAnnotations();
    }
  }

  async offerToDebug() {
    const launch = vscode.workspace.getConfiguration("launch", this.folder);
    const configurations = launch.inspect("configurations")?.workspaceFolderValue ?? [];
    const isStillInstalled = vscode.extensions.getExtension(DEFOLD_KIT);
    if (isStillInstalled || !configurations.some(wasLeftByDefoldKit)) {
      return;
    }
    const replace = "Replace it";
    const chosen = await vscode.window.showInformationMessage(
      `${this.folder.name} starts the game through Defold Kit, which is not installed, so F5 fails. Replacing that launch configuration with the one of Defold Tools writes .vscode/launch.json.`,
      replace,
      "Not now",
    );
    if (chosen === replace) {
      const kept = configurations.filter((configuration) => !wasLeftByDefoldKit(configuration));
      await launch.update("configurations", [...kept, LAUNCH_CONFIGURATION], vscode.ConfigurationTarget.WorkspaceFolder);
      this.log.appendLine(`F5 in ${this.folder.name} now starts the game with Defold Tools`);
    }
  }

  async useAnnotations() {
    if (!this.annotations) {
      vscode.window.showWarningMessage("The annotations are not ready yet.");
      return;
    }
    const kept = this.library().filter((entry) => !ANNOTATIONS_OF_OTHERS.test(entry));
    const settings = vscode.workspace.getConfiguration("Lua", this.folder);
    await settings.update(LIBRARY_SETTING, [...kept, this.annotations.directory], vscode.ConfigurationTarget.WorkspaceFolder);
    this.log.appendLine(`Lua.workspace.library of ${this.folder.name} now points at ${this.annotations.directory}`);
    if (!this.declaresARuntime()) {
      await settings.update(RUNTIME_SETTING, RUNTIME_OF_DEFOLD, vscode.ConfigurationTarget.WorkspaceFolder);
      this.log.appendLine(`Lua.runtime.version of ${this.folder.name} is now ${RUNTIME_OF_DEFOLD}`);
    }
  }

  stopTheGame() {
    this.game?.terminate();
    const session = vscode.debug.activeDebugSession;
    if (session?.type === DEBUGGER) {
      vscode.debug.stopDebugging(session);
    }
  }

  async editsThatFollow(moved) {
    const files = moved
      .filter((each) => this.holds(each.oldUri) && this.holds(each.newUri))
      .map((each) => ({ oldUri: each.oldUri.toString(), newUri: each.newUri.toString() }));
    const edit = new vscode.WorkspaceEdit();
    if (files.length === 0) {
      return edit;
    }
    const answered = await this.ask("workspace/willRenameFiles", { files }).catch(() => null);
    Object.entries(answered?.changes ?? {}).forEach(([uri, edits]) => {
      edits.forEach((each) => edit.replace(asUri(uri), asRange(each.range), each.newText));
    });
    return edit;
  }

  holds(uri) {
    const folder = vscode.workspace.getWorkspaceFolder(uri);
    return uri.scheme === "file" && (folder?.uri.toString() === this.folder.uri.toString() || uri.fsPath.startsWith(this.folder.uri.fsPath + path.sep));
  }

  run(...asked) {
    const program = vscode.workspace.getConfiguration("defoldTools", this.folder).get("server.path");
    return new Promise((resolve, reject) => {
      execFile(program, asked, { cwd: this.folder.uri.fsPath }, (error, stdout, stderr) => {
        if (error) {
          reject(new Error((stderr || error.message).trim()));
        } else {
          resolve(stdout);
        }
      });
    });
  }

  async makeAFile(where) {
    const kinds = JSON.parse(await this.run("new", "--kinds"));
    const offered = kinds.flatMap((kind) => {
      const from = kind.madeFrom.length ? `made from ${kind.madeFrom.join(" or ")}` : "";
      const plain = { label: kind.extension, description: kind.extension === ".atlas" ? "empty" : from, kind };
      const more = [];
      if (kind.takesAScript) {
        more.push({ label: kind.extension, description: "with its script", kind, withScript: true });
      }
      if (kind.extension === ".atlas") {
        more.push({ label: kind.extension, description: "with the images under its directory", kind, withImages: true });
      }
      return [plain, ...more];
    });
    const chosen = await vscode.window.showQuickPick(offered, { placeHolder: "The kind of file to make", matchOnDescription: true });
    if (!chosen) {
      return;
    }
    const directory = path.relative(this.folder.uri.fsPath, where).split(path.sep).join("/");
    const start = directory ? `${directory}/` : "";
    const named = await vscode.window.showInputBox({
      prompt: `The ${chosen.label} file to make, from the project's directory`,
      value: `${start}name${chosen.label}`,
      valueSelection: [start.length, start.length + "name".length],
    });
    if (!named) {
      return;
    }
    const asked = ["new"];
    if (chosen.withScript) {
      asked.push("--script");
    }
    if (chosen.withImages) {
      asked.push("--from", path.dirname(path.join(this.folder.uri.fsPath, named)));
    }
    if (chosen.kind.madeFrom.length) {
      const extensions = chosen.kind.madeFrom.map((extension) => extension.slice(1)).join(",");
      const found = await vscode.workspace.findFiles(new vscode.RelativePattern(this.folder, `**/*.{${extensions}}`), DIRECTORIES_NOT_SEARCHED, MOST_FILES_OFFERED);
      const candidates = found.map((uri) => ({ label: vscode.workspace.asRelativePath(uri, false), uri })).sort((first, second) => first.label.localeCompare(second.label));
      const from = await vscode.window.showQuickPick([{ label: "Leave it to be filled in" }, ...candidates], { placeHolder: `What the ${chosen.label} file is made from` });
      if (!from) {
        return;
      }
      if (from.uri) {
        asked.push("--from", from.uri.fsPath);
      }
    }
    try {
      const made = (await this.run(...asked, path.join(this.folder.uri.fsPath, named))).trim().split(/\r?\n/);
      await vscode.window.showTextDocument(vscode.Uri.file(path.join(this.folder.uri.fsPath, made[0])));
    } catch (failure) {
      vscode.window.showErrorMessage(failure.message);
    }
  }

  async showWhatUses(document) {
    const found = (await this.ask("defold/uses", { textDocument: { uri: document.uri.toString() } }).catch(() => null)) ?? [];
    if (found.length === 0) {
      vscode.window.showInformationMessage(`Nothing in ${this.folder.name} names ${path.basename(document.uri.fsPath)}.`);
      return;
    }
    const places = found.map((where) => new vscode.Location(asUri(where.uri), asRange(where.range)));
    vscode.commands.executeCommand("editor.action.showReferences", document.uri, new vscode.Position(0, 0), places);
  }

  openReference() {
    if (this.annotations) {
      vscode.env.openExternal(vscode.Uri.parse(this.annotations.reference));
    }
  }

  becomeReady() {
    this.isReady = true;
    this.send({ method: "initialized", params: {} });
    this.offerToDebug();
    vscode.workspace.textDocuments.forEach((document) => this.open(document));
    const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(this.folder, "**/*"));
    const tell = (uri) => this.tellOfFiles(uri);
    this.subscriptions.push(
      watcher,
      watcher.onDidChange(tell),
      watcher.onDidCreate(tell),
      watcher.onDidDelete(tell),
      vscode.workspace.onDidOpenTextDocument((document) => this.open(document)),
      vscode.workspace.onDidChangeTextDocument((event) => this.change(event.document)),
      vscode.workspace.onDidCloseTextDocument((document) => this.close(document)),
    );
    this.provide();
  }

  provide() {
    const scripts = { scheme: "file", pattern: new vscode.RelativePattern(this.folder, SCRIPTS) };
    const settings = { scheme: "file", pattern: new vscode.RelativePattern(this.folder, SETTINGS) };
    const data = { scheme: "file", pattern: new vscode.RelativePattern(this.folder, DATA) };
    const both = [scripts, settings];
    const completions = {
      provideCompletionItems: async (document, position) => {
        const items = (await this.askQuietly("textDocument/completion", document, position)) ?? [];
        return items.map((offered) => {
          const item = new vscode.CompletionItem(offered.label, offered.kind - 1);
          item.detail = offered.detail;
          item.documentation = offered.documentation && new vscode.MarkdownString(offered.documentation.value);
          item.filterText = offered.filterText;
          item.sortText = offered.sortText;
          item.insertText = offered.textEdit.newText;
          item.range = asRange(offered.textEdit.range);
          return item;
        });
      },
    };
    this.subscriptions.push(
      vscode.languages.registerCompletionItemProvider(scripts, completions, ...CHARACTERS_THAT_START_A_NAME),
      vscode.languages.registerCompletionItemProvider(settings, completions, ...CHARACTERS_THAT_START_A_SETTING),
      vscode.languages.registerCompletionItemProvider(data, completions, ...CHARACTERS_THAT_START_A_VALUE),
      vscode.languages.registerDefinitionProvider([...both, data], {
        provideDefinition: async (document, position) => {
          const found = (await this.askQuietly("textDocument/definition", document, position)) ?? [];
          return found.map((where) => new vscode.Location(asUri(where.uri), asRange(where.range)));
        },
      }),
      vscode.languages.registerHoverProvider([...both, data], {
        provideHover: async (document, position) => {
          const said = await this.askQuietly("textDocument/hover", document, position);
          return said ? new vscode.Hover(new vscode.MarkdownString(said.contents.value), asRange(said.range)) : undefined;
        },
      }),
      vscode.languages.registerReferenceProvider([scripts, data], {
        provideReferences: async (document, position) => {
          const found = (await this.askQuietly("textDocument/references", document, position)) ?? [];
          return found.map((where) => new vscode.Location(asUri(where.uri), asRange(where.range)));
        },
      }),
      vscode.languages.registerRenameProvider(scripts, {
        prepareRename: async (document, position) => {
          const offered = await this.askAbout("textDocument/prepareRename", document, position);
          return offered ? { range: asRange(offered.range), placeholder: offered.placeholder } : undefined;
        },
        provideRenameEdits: async (document, position, newName) => {
          const answered = await this.askAbout("textDocument/rename", document, position, undefined, { newName });
          const edit = new vscode.WorkspaceEdit();
          Object.entries(answered?.changes ?? {}).forEach(([uri, edits]) => {
            edits.forEach((each) => edit.replace(asUri(uri), asRange(each.range), each.newText));
          });
          return edit;
        },
      }),
      vscode.languages.registerCodeActionsProvider(
        [...both, data],
        {
          provideCodeActions: async (document, range, context) => {
            const offered = (await this.askQuietly("textDocument/codeAction", document, undefined, range)) ?? [];
            return offered.map((fix) => {
              const action = new vscode.CodeAction(fix.title, vscode.CodeActionKind.QuickFix);
              action.isPreferred = fix.isPreferred;
              action.edit = new vscode.WorkspaceEdit();
              Object.entries(fix.edit.changes).forEach(([uri, edits]) => {
                edits.forEach((edit) => action.edit.replace(asUri(uri), asRange(edit.range), edit.newText));
              });
              action.diagnostics = context.diagnostics.filter((shown) =>
                fix.diagnostics.some((fixed) => fixed.code === shown.code && fixed.range.start.line === shown.range.start.line),
              );
              return action;
            });
          },
        },
        { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix] },
      ),
    );
  }

  owns(document) {
    if (!this.isReady || document.uri.scheme !== "file") {
      return false;
    }
    const folder = vscode.workspace.getWorkspaceFolder(document.uri);
    return folder !== undefined && folder.uri.toString() === this.folder.uri.toString();
  }

  open(document) {
    if (this.owns(document)) {
      this.send({ method: "textDocument/didOpen", params: { textDocument: { uri: document.uri.toString(), text: document.getText() } } });
    }
  }

  change(document) {
    if (this.owns(document)) {
      this.send({
        method: "textDocument/didChange",
        params: { textDocument: { uri: document.uri.toString() }, contentChanges: [{ text: document.getText() }] },
      });
    }
  }

  close(document) {
    if (this.owns(document)) {
      this.send({ method: "textDocument/didClose", params: { textDocument: { uri: document.uri.toString() } } });
    }
  }

  tellOfFiles(uri) {
    if (DIRECTORIES_THE_BUILD_WRITES.test(uri.fsPath)) {
      return;
    }
    clearTimeout(this.tellingOfFiles);
    this.tellingOfFiles = setTimeout(() => {
      this.send({ method: "workspace/didChangeWatchedFiles", params: { changes: [] } });
    }, PAUSE_BEFORE_TELLING_OF_FILES);
  }

  show(published) {
    const uri = asUri(published.uri);
    const shown = published.diagnostics.map((found) => {
      const range = new vscode.Range(found.range.start.line, found.range.start.character, found.range.end.line, found.range.end.character);
      const diagnostic = new vscode.Diagnostic(range, found.message, SEVERITIES[found.severity] ?? vscode.DiagnosticSeverity.Error);
      diagnostic.source = found.source;
      diagnostic.code = found.code;
      return diagnostic;
    });
    this.diagnostics.set(uri, shown);
    if (shown.length > 0) {
      this.reported.add(uri.toString());
    } else {
      this.reported.delete(uri.toString());
    }
  }
}

function asUri(written) {
  return vscode.Uri.file(vscode.Uri.parse(written).fsPath);
}

function asRange(written) {
  return new vscode.Range(written.start.line, written.start.character, written.end.line, written.end.character);
}

function asWritten(range) {
  return {
    start: { line: range.start.line, character: range.start.character },
    end: { line: range.end.line, character: range.end.character },
  };
}

const servers = new Map();

function startFor(folder, diagnostics, log) {
  if (servers.has(folder.uri.toString()) || !fs.existsSync(path.join(folder.uri.fsPath, PROJECT_FILE))) {
    return;
  }
  const server = new Server(folder, diagnostics, log);
  servers.set(folder.uri.toString(), server);
  server.start();
}

function stopAll() {
  servers.forEach((server) => server.stop());
  servers.clear();
}

function activate(context) {
  const diagnostics = vscode.languages.createDiagnosticCollection("defold-vet");
  const log = vscode.window.createOutputChannel("Defold Tools");
  const first = () => servers.values().next().value;
  const startAll = () => (vscode.workspace.workspaceFolders ?? []).forEach((folder) => startFor(folder, diagnostics, log));
  const restart = () => {
    stopAll();
    startAll();
  };
  const fileView = new FileView(context, log);
  context.subscriptions.push(
    diagnostics,
    log,
    ...fileView.register(),
    vscode.commands.registerCommand("defoldTools.restart", restart),
    vscode.commands.registerCommand("defoldTools.build", () => first()?.play("build")),
    vscode.commands.registerCommand("defoldTools.run", () => first()?.play("run")),
    vscode.commands.registerCommand("defoldTools.resolve", () => first()?.play("resolve")),
    vscode.commands.registerCommand("defoldTools.clean", () => first()?.play("clean")),
    vscode.commands.registerCommand("defoldTools.stop", () => first()?.stopTheGame()),
    vscode.commands.registerCommand("defoldTools.debug", () => {
      const server = first();
      if (server) {
        vscode.debug.startDebugging(server.folder, LAUNCH_CONFIGURATION);
      }
    }),
    vscode.debug.registerDebugConfigurationProvider(DEBUGGER, {
      resolveDebugConfiguration: (folder, configuration) => {
        const server = (folder && servers.get(folder.uri.toString())) ?? first();
        if (!server) {
          return undefined;
        }
        const settings = vscode.workspace.getConfiguration("defoldTools", server.folder);
        return {
          ...LAUNCH_CONFIGURATION,
          ...configuration,
          root: configuration.root ?? server.folder.uri.fsPath,
          defoldHome: configuration.defoldHome ?? settings.get("defoldHome"),
        };
      },
    }),
    vscode.debug.registerDebugAdapterDescriptorFactory(DEBUGGER, {
      createDebugAdapterDescriptor: (session) => {
        const settings = vscode.workspace.getConfiguration("defoldTools", session.workspaceFolder);
        return new vscode.DebugAdapterExecutable(settings.get("server.path"), ["dap"], { cwd: session.configuration.root });
      },
    }),
    vscode.commands.registerCommand("defoldTools.syncAtlas", () => {
      const document = vscode.window.activeTextEditor?.document;
      if (document?.uri.fsPath.endsWith(".atlas")) {
        document.save().then(() => first()?.play("atlas", document.uri.fsPath));
      } else {
        vscode.window.showWarningMessage("Open the atlas to bring in line with the images on disk first.");
      }
    }),
    vscode.commands.registerCommand("defoldTools.reload", () => {
      const document = vscode.window.activeTextEditor?.document;
      if (document) {
        document.save().then(() => servers.forEach((server) => server.reload(document)));
      }
    }),
    vscode.tasks.onDidEndTask((ended) => servers.forEach((server) => server.hasStopped(ended.execution))),
    vscode.workspace.onDidSaveTextDocument((document) => {
      servers.forEach((server) => {
        if (vscode.workspace.getConfiguration("defoldTools", server.folder).get("reloadOnSave")) {
          server.reload(document);
        }
      });
    }),
    vscode.commands.registerCommand("defoldTools.openReference", () => first()?.openReference()),
    vscode.commands.registerCommand("defoldTools.bundle", async () => {
      const server = first();
      if (!server) {
        vscode.window.showWarningMessage("Open a Defold project to bundle first.");
        return;
      }
      const offered = PLATFORMS_TO_BUNDLE_FOR.map((each) => ({ ...each, description: each.platform }));
      const chosen = await vscode.window.showQuickPick(offered, { placeHolder: "The platform to bundle for", matchOnDescription: true });
      const variant = chosen && (await vscode.window.showQuickPick(["release", "debug"], { placeHolder: "The variant of the engine" }));
      if (variant) {
        server.play("bundle", "--platform", chosen.platform, "--variant", variant);
      }
    }),
    vscode.commands.registerCommand("defoldTools.new", (clicked) => {
      const active = vscode.window.activeTextEditor?.document.uri;
      const near = clicked ?? active;
      const server = (near && [...servers.values()].find((each) => each.holds(near))) ?? first();
      if (!server) {
        vscode.window.showWarningMessage("Open a Defold project to make a file in first.");
        return;
      }
      const isADirectory = near && fs.existsSync(near.fsPath) && fs.statSync(near.fsPath).isDirectory();
      const where = near && server.holds(near) ? (isADirectory ? near.fsPath : path.dirname(near.fsPath)) : server.folder.uri.fsPath;
      server.makeAFile(where);
    }),
    vscode.commands.registerCommand("defoldTools.uses", () => {
      const document = vscode.window.activeTextEditor?.document;
      const server = document && [...servers.values()].find((each) => each.holds(document.uri));
      if (server) {
        server.showWhatUses(document);
      } else {
        vscode.window.showWarningMessage("Open a file of a Defold project to see what uses it first.");
      }
    }),
    vscode.workspace.onWillRenameFiles((event) => {
      event.waitUntil(
        Promise.all([...servers.values()].map((server) => server.editsThatFollow(event.files))).then((edits) => {
          const all = new vscode.WorkspaceEdit();
          edits.forEach((each) => each.entries().forEach(([uri, changes]) => all.set(uri, changes)));
          return all;
        }),
      );
    }),
    vscode.commands.registerCommand("defoldTools.useAnnotations", () => first()?.useAnnotations()),
    vscode.workspace.onDidChangeWorkspaceFolders(restart),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("defoldTools")) {
        restart();
      }
    }),
  );
  startAll();
  fileView.follow(vscode.window.activeTextEditor);
}

function deactivate() {
  stopAll();
}

module.exports = { activate, deactivate };
