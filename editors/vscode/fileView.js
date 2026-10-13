"use strict";

const vscode = require("vscode");
const { execFile } = require("child_process");
const path = require("path");

const KINDS = {
  ".atlas": { command: "pack", usesTheEditor: true, followsTyping: false },
  ".tilesource": { command: "tiles", usesTheEditor: false, followsTyping: true },
  ".tilemap": { command: "tiles", usesTheEditor: false, followsTyping: true },
  ".collection": { command: "scene", usesTheEditor: false, followsTyping: true },
  ".gui": { command: "scene", usesTheEditor: false, followsTyping: true },
  ".go": { command: "scene", usesTheEditor: false, followsTyping: true, mayBeSomethingElse: true },
};
const WATCHED = "**/*.{png,jpg,jpeg,atlas,tilesource,tilemap,go,collection,sprite,label,collisionobject,gui,font,project}";
const VIEW = "defoldTools.view";
const PAUSE_BEFORE_SHOWING_AGAIN = 300;
const GAP_BETWEEN_PAGES = 16;
const HOW_MUCH_BETTER_BELOW_HAS_TO_BE = 1.15;
const SIDE_BY_SIDE = 0;
const ONE_OVER_THE_OTHER = 1;
const LONGEST_OUTPUT = 64 * 1024 * 1024;

function kindOf(document) {
  return document?.uri.scheme === "file" ? KINDS[path.extname(document.uri.fsPath)] : undefined;
}

function isTheSameFile(first, second) {
  const written = (file) => (process.platform === "win32" ? path.normalize(file).toLowerCase() : path.normalize(file));
  return written(first) === written(second);
}

function extentOf(shown) {
  if (shown.kind === "scene") {
    return { width: Math.max(1, shown.maxX - shown.minX), height: Math.max(1, shown.maxY - shown.minY) };
  }
  if (shown.kind === "tilemap") {
    const bounds = shown.bounds ?? { minX: 0, minY: 0, maxX: 0, maxY: 0 };
    return { width: (bounds.maxX - bounds.minX + 1) * shown.source.tileWidth, height: (bounds.maxY - bounds.minY + 1) * shown.source.tileHeight };
  }
  if (shown.kind === "tilesource") {
    return { width: shown.source.width, height: shown.source.height };
  }
  return { width: shown.pages * shown.width + (shown.pages - 1) * GAP_BETWEEN_PAGES, height: shown.height };
}

function whereTheViewFitsBest(view, opened, extent) {
  const area = opened === "down" ? { width: view.width, height: view.height * 2 } : { width: view.width * 2, height: view.height };
  const beside = Math.min(area.width / 2 / extent.width, area.height / extent.height);
  const below = Math.min(area.width / extent.width, area.height / 2 / extent.height);
  return below > beside * HOW_MUCH_BETTER_BELOW_HAS_TO_BE ? "down" : "right";
}

class FileView {
  constructor(context, log) {
    this.context = context;
    this.log = log;
    this.seen = new Set();
    this.asked = 0;
  }

  register() {
    const watched = vscode.workspace.createFileSystemWatcher(WATCHED);
    const changed = (uri) => this.showAgainFor(uri);
    return [
      watched,
      watched.onDidChange(changed),
      watched.onDidCreate(changed),
      watched.onDidDelete(changed),
      vscode.commands.registerCommand("defoldTools.show", () => this.showTheActiveOne()),
      vscode.window.onDidChangeActiveTextEditor((editor) => this.follow(editor)),
      vscode.window.onDidChangeTextEditorSelection((event) => this.pointAt(event.textEditor)),
      vscode.workspace.onDidSaveTextDocument((document) => {
        if (this.isShowing(document)) {
          this.ask();
        }
      }),
      vscode.workspace.onDidChangeTextDocument((event) => {
        if (this.isShowing(event.document) && kindOf(event.document).followsTyping) {
          this.askSoon();
        }
      }),
      vscode.workspace.onDidCloseTextDocument((document) => this.seen.delete(document.uri.toString())),
    ];
  }

  isShowing(document) {
    return Boolean(this.panel) && document.uri.toString() === this.file?.toString();
  }

  showTheActiveOne() {
    const document = vscode.window.activeTextEditor?.document;
    if (!kindOf(document)) {
      vscode.window.showWarningMessage("Open the atlas, the tiles, the game object or the collection to show first.");
      return;
    }
    this.show(document);
  }

  follow(editor) {
    const document = editor?.document;
    if (!kindOf(document) || !vscode.workspace.getWorkspaceFolder(document.uri)) {
      return;
    }
    const isNew = !this.seen.has(document.uri.toString());
    const opensWithTheFile = vscode.workspace.getConfiguration("defoldTools", document.uri).get("showWithTheFile");
    if (this.panel ? this.isShowing(document) : !isNew || !opensWithTheFile) {
      return;
    }
    if (!kindOf(document).mayBeSomethingElse) {
      this.show(document);
      return;
    }
    this.run(document.uri, (failure, shown) => {
      if (!failure && vscode.window.activeTextEditor?.document.uri.toString() === document.uri.toString()) {
        this.show(document, shown);
      }
    });
  }

  show(document, already) {
    this.seen.add(document.uri.toString());
    this.file = document.uri;
    if (!this.panel) {
      this.open();
    }
    this.panel.title = path.basename(document.uri.fsPath);
    if (already) {
      this.asked++;
      this.take(already, "");
    } else {
      this.ask();
    }
  }

  open() {
    const media = vscode.Uri.joinPath(this.context.extensionUri, "media");
    const folders = (vscode.workspace.workspaceFolders ?? []).map((folder) => folder.uri);
    this.panel = vscode.window.createWebviewPanel(
      VIEW,
      "Defold",
      { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
      { enableScripts: true, retainContextWhenHidden: true, localResourceRoots: [media, ...folders] },
    );
    this.isPlaced = false;
    this.size = undefined;
    this.shown = undefined;
    const webview = this.panel.webview;
    const source = webview.cspSource;
    webview.html = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${source} data:; style-src ${source}; script-src ${source};">
<link rel="stylesheet" href="${webview.asWebviewUri(vscode.Uri.joinPath(media, "view.css"))}">
<title>Defold</title>
</head>
<body>
<header><span id="summary"></span><span id="status"></span></header>
<main id="stage"><div id="canvas"></div></main>
<footer id="about"></footer>
<script src="${webview.asWebviewUri(vscode.Uri.joinPath(media, "view.js"))}"></script>
</body>
</html>`;
    webview.onDidReceiveMessage((message) => this.hear(message));
    this.panel.onDidDispose(() => {
      this.panel = undefined;
      this.shown = undefined;
    });
  }

  hear(message) {
    if (message.size) {
      this.size = message.size;
      this.place();
    } else if (message.reveal) {
      this.reveal(message.reveal);
    }
  }

  place() {
    if (this.isPlaced || !this.size || !this.shown) {
      return;
    }
    this.isPlaced = true;
    const opened = vscode.workspace.getConfiguration("workbench.editor").get("openSideBySideDirection") === "down" ? "down" : "right";
    const best = whereTheViewFitsBest(this.size, opened, extentOf(this.shown));
    if (best !== opened && vscode.window.tabGroups.all.length === 2) {
      vscode.commands.executeCommand("vscode.setEditorLayout", {
        orientation: best === "down" ? ONE_OVER_THE_OTHER : SIDE_BY_SIDE,
        groups: [{}, {}],
      });
    }
  }

  async reveal(line) {
    const shown = vscode.window.visibleTextEditors.find((editor) => editor.document.uri.toString() === this.file?.toString());
    const editor = shown ?? (await vscode.window.showTextDocument(this.file, { viewColumn: vscode.ViewColumn.One, preserveFocus: true }));
    const written = editor.document.lineAt(Math.min(line - 1, editor.document.lineCount - 1)).range;
    editor.selection = new vscode.Selection(written.start, written.end);
    editor.revealRange(written, vscode.TextEditorRevealType.InCenterIfOutsideViewport);
  }

  pointAt(editor) {
    if (this.isShowing(editor.document)) {
      this.panel.webview.postMessage({ line: editor.selection.active.line + 1 });
    }
  }

  showAgainFor(changed) {
    if (!this.panel || !this.shown || changed.toString() === this.file.toString()) {
      return;
    }
    const shown = this.shown;
    const uses = shown.uses ?? (shown.source ? [shown.source.image, shown.source.file] : shown.images.map((placed) => placed.image));
    if (uses.some((used) => isTheSameFile(path.join(shown.root, used), changed.fsPath))) {
      this.askSoon();
    }
  }

  askSoon() {
    clearTimeout(this.pending);
    this.pending = setTimeout(() => this.ask(), PAUSE_BEFORE_SHOWING_AGAIN);
  }

  addressed(shown) {
    const addressOf = (image) => `${this.panel.webview.asWebviewUri(vscode.Uri.file(path.join(shown.root, image)))}?${Date.now()}`;
    if (shown.kind === "scene") {
      return { ...shown, images: shown.images.map((image) => ({ path: image.path, address: image.data ?? addressOf(image.path) })) };
    }
    if (shown.source) {
      return { ...shown, source: { ...shown.source, address: shown.source.imageData ?? addressOf(shown.source.image), imageData: undefined } };
    }
    return { ...shown, kind: "atlas", images: shown.images.map((placed) => ({ ...placed, source: addressOf(placed.image) })) };
  }

  take(shown, status) {
    this.shown = shown;
    this.panel.webview.postMessage({ shown: this.addressed(shown), status });
    this.place();
  }

  run(file, done) {
    const kind = KINDS[path.extname(file.fsPath)];
    const settings = vscode.workspace.getConfiguration("defoldTools", file);
    const home = settings.get("defoldHome");
    const held = vscode.workspace.textDocuments.find((document) => document.uri.toString() === file.toString());
    const typed = kind.followsTyping && held ? held.getText() : undefined;
    const running = execFile(
      settings.get("server.path"),
      [kind.command, ...(home && kind.usesTheEditor ? ["--defold-home", home] : []), ...(typed === undefined ? [] : ["--stdin"]), file.fsPath],
      { cwd: path.dirname(file.fsPath), maxBuffer: LONGEST_OUTPUT },
      (error, stdout, stderr) => {
        if (error) {
          done((stderr || error.message).trim());
          return;
        }
        try {
          done(undefined, JSON.parse(stdout));
        } catch (unreadable) {
          done(`what ${kind.command} said could not be read: ${unreadable.message}`);
        }
      },
    );
    if (typed !== undefined) {
      running.stdin.end(typed);
    }
  }

  ask() {
    const file = this.file;
    const kind = KINDS[path.extname(file.fsPath)];
    const asked = ++this.asked;
    const held = vscode.workspace.textDocuments.find((document) => document.uri.toString() === file.toString());
    const isBehind = !kind.followsTyping && Boolean(held?.isDirty);
    if (kind.usesTheEditor) {
      this.panel.webview.postMessage({ status: "Packing…" });
    }
    this.run(file, (failure, shown) => {
      if (!this.panel || asked !== this.asked) {
        return;
      }
      if (failure) {
        this.log.appendLine(`showing ${file.fsPath}: ${failure}`);
        this.panel.webview.postMessage({ failure });
        return;
      }
      this.take(shown, isBehind ? "What is not saved yet is not shown" : "");
    });
  }
}

module.exports = { FileView, kindOf, extentOf, whereTheViewFitsBest };
