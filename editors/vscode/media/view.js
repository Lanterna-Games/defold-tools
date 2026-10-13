"use strict";

(function () {
  const host = acquireVsCodeApi();
  const stage = document.getElementById("stage");
  const canvas = document.getElementById("canvas");
  const summary = document.getElementById("summary");
  const status = document.getElementById("status");
  const about = document.getElementById("about");

  const GAP_BETWEEN_PAGES = 16;
  const ROOM_AROUND = 24;
  const SMALLEST_ZOOM = 0.02;
  const LARGEST_ZOOM = 64;
  const ZOOM_OF_A_NOTCH = 1.15;
  const A_DRAG_STARTS_AFTER = 3;
  const PAUSE_BEFORE_SAYING_THE_SIZE = 150;
  const LARGEST_SIDE_OF_A_DRAWING = 8192;
  const SMALLEST_TILE_WITH_ITS_NUMBER = 26;
  const ACROSS = 1;
  const UP_DOWN = 2;
  const TURNED = 4;

  let shown;
  let extent = { width: 1, height: 1 };
  let zoom = 1;
  let offset = { x: 0, y: 0 };
  let isFitted = true;
  let pointed = 0;
  let point = () => {};
  let whatIsAt = () => undefined;
  let redraw = () => {};
  let scene;

  function made(tag, className, style) {
    const element = document.createElement(tag);
    element.className = className ?? "";
    Object.entries(style ?? {}).forEach(([name, value]) => {
      element.style[name] = typeof value === "number" ? `${value}px` : value;
    });
    return element;
  }

  function counted(count, one, many) {
    return count === 1 ? `1 ${one}` : `${count} ${many}`;
  }

  function apply() {
    canvas.style.transform = `translate(${offset.x}px, ${offset.y}px) scale(${zoom})`;
    canvas.style.setProperty("--line", `${1 / zoom}px`);
    canvas.classList.toggle("crisp", zoom >= 1);
    canvas.classList.toggle("numbered", shown?.kind === "tilesource" && shown.source.tileWidth * zoom >= SMALLEST_TILE_WITH_ITS_NUMBER);
    redraw();
  }

  function fit() {
    if (!shown) {
      return;
    }
    const room = { width: stage.clientWidth - 2 * ROOM_AROUND, height: stage.clientHeight - 2 * ROOM_AROUND };
    zoom = Math.max(SMALLEST_ZOOM, Math.min(LARGEST_ZOOM, room.width / extent.width, room.height / extent.height));
    offset = { x: (stage.clientWidth - extent.width * zoom) / 2, y: (stage.clientHeight - extent.height * zoom) / 2 };
    isFitted = true;
    apply();
  }

  function drawAtlas() {
    extent = { width: shown.pages * shown.width + (shown.pages - 1) * GAP_BETWEEN_PAGES, height: shown.height };
    const pages = [];
    for (let index = 0; index < shown.pages; index++) {
      const page = made("div", "page", { left: index * (shown.width + GAP_BETWEEN_PAGES), width: shown.width, height: shown.height });
      canvas.appendChild(page);
      pages.push(page);
    }
    shown.images.forEach((placed) => {
      const frame = made("div", "image", { left: placed.x, top: placed.y, width: placed.width, height: placed.height });
      frame.dataset.line = placed.line;
      const picture = made("img", "", placed.rotated ? { width: placed.height, height: placed.width, transform: `translateX(${placed.width}px) rotate(90deg)` } : { width: placed.width, height: placed.height });
      picture.src = placed.source;
      picture.alt = placed.name;
      picture.draggable = false;
      frame.appendChild(picture);
      const turned = placed.rotated ? `${placed.height} × ${placed.width}, turned` : `${placed.width} × ${placed.height}`;
      const of = placed.animation ? ` · animation ${placed.animation}` : "";
      const page = shown.pages > 1 ? ` · page ${placed.page + 1}` : "";
      frame.dataset.about = `${placed.name}${of} · ${turned} · at ${placed.x}, ${placed.y}${page} · ${placed.image}`;
      (pages[placed.page] ?? pages[0]).appendChild(frame);
    });
    const pagesSaid = shown.pages > 1 ? ` · ${shown.pages} pages` : "";
    summary.textContent = `${shown.width} × ${shown.height}${pagesSaid} · ${counted(shown.images.length, "image", "images")} · Defold ${shown.defold}`;
    point = (line) => canvas.querySelectorAll(".image").forEach((frame) => frame.classList.toggle("pointed", Number(frame.dataset.line) === line));
    whatIsAt = (event) => {
      const frame = event.target.closest(".image");
      return frame ? { about: frame.dataset.about, line: Number(frame.dataset.line) } : undefined;
    };
  }

  function drawTileSource() {
    const source = shown.source;
    extent = { width: source.width, height: source.height };
    const page = made("div", "page", { left: 0, width: source.width, height: source.height });
    const picture = made("img", "whole", { width: source.width, height: source.height });
    picture.src = source.address;
    picture.alt = source.image;
    picture.draggable = false;
    page.appendChild(picture);
    source.tiles.forEach(([x, y], index) => {
      const of = source.animations.filter((animation) => index >= animation.first && index <= animation.last);
      const tile = made("div", of.length ? "tile animated" : "tile", { left: x, top: y, width: source.tileWidth, height: source.tileHeight });
      tile.dataset.tile = index;
      tile.dataset.number = index + 1;
      const animations = of.length ? ` · ${of.map((animation) => `animation ${animation.id}`).join(", ")}` : "";
      tile.dataset.about = `tile ${index + 1} · column ${(index % source.columns) + 1}, row ${Math.floor(index / source.columns) + 1} · at ${x}, ${y}${animations}`;
      if (of.length) {
        tile.dataset.line = of[0].line;
      }
      page.appendChild(tile);
    });
    canvas.appendChild(page);
    const animationsSaid = source.animations.length ? ` · ${counted(source.animations.length, "animation", "animations")}` : "";
    summary.textContent = `${source.width} × ${source.height} · ${counted(source.tiles.length, "tile", "tiles")} of ${source.tileWidth} × ${source.tileHeight}, ${source.columns} by ${source.rows}${animationsSaid}`;
    point = (line) => {
      const animation = source.animations.find((each) => line >= each.line && line <= each.lastLine);
      canvas.querySelectorAll(".tile").forEach((tile) => {
        const index = Number(tile.dataset.tile);
        tile.classList.toggle("pointed", Boolean(animation) && index >= animation.first && index <= animation.last);
      });
    };
    whatIsAt = (event) => {
      const tile = event.target.closest(".tile");
      return tile ? { about: tile.dataset.about, line: Number(tile.dataset.line ?? 0) } : undefined;
    };
  }

  function drawTileMap() {
    const source = shown.source;
    const bounds = shown.bounds;
    const layers = [...(shown.layers ?? [])].sort((first, second) => first.z - second.z);
    const visible = layers.filter((layer) => layer.visible);
    const without = shown.cellsWithoutATile ? ` · ${counted(shown.cellsWithoutATile, "cell names", "cells name")} a tile the source does not have` : "";
    const hidden = layers.length - visible.length ? `, ${layers.length - visible.length} hidden` : "";
    if (!bounds) {
      extent = { width: source.tileWidth, height: source.tileHeight };
      summary.textContent = `No cell yet · ${counted(layers.length, "layer", "layers")}${hidden}${without}`;
      return;
    }
    const across = bounds.maxX - bounds.minX + 1;
    const down = bounds.maxY - bounds.minY + 1;
    extent = { width: across * source.tileWidth, height: down * source.tileHeight };
    const shrink = Math.max(1, Math.ceil(Math.max(extent.width, extent.height) / LARGEST_SIDE_OF_A_DRAWING));
    const drawing = made("canvas", "map", { width: extent.width, height: extent.height });
    drawing.width = Math.ceil(extent.width / shrink);
    drawing.height = Math.ceil(extent.height / shrink);
    const page = made("div", "page plain", { left: 0, width: extent.width, height: extent.height });
    const pointer = made("div", "cell");
    page.append(drawing, pointer);
    canvas.appendChild(page);
    const placeOf = (x, y) => ({ left: (x - bounds.minX) * source.tileWidth, top: (bounds.maxY - y) * source.tileHeight });
    const onTop = new Map();
    visible.forEach((layer) => layer.cells.forEach((cell) => onTop.set(`${cell[0]},${cell[1]}`, { layer, cell })));
    const picture = new Image();
    picture.onload = () => {
      const context = drawing.getContext("2d");
      context.imageSmoothingEnabled = false;
      context.scale(1 / shrink, 1 / shrink);
      visible.forEach((layer) =>
        layer.cells.forEach(([x, y, tile, done]) => {
          const [fromX, fromY] = source.tiles[tile];
          const at = placeOf(x, y);
          context.save();
          context.translate(at.left + source.tileWidth / 2, at.top + source.tileHeight / 2);
          const isTurned = Boolean(done & TURNED);
          if (isTurned) {
            context.rotate(Math.PI / 2);
          }
          context.scale(done & ACROSS ? -1 : 1, done & UP_DOWN ? -1 : 1);
          const wide = isTurned ? source.tileHeight : source.tileWidth;
          const high = isTurned ? source.tileWidth : source.tileHeight;
          context.drawImage(picture, fromX, fromY, source.tileWidth, source.tileHeight, -wide / 2, -high / 2, wide, high);
          context.restore();
        }),
      );
    };
    picture.src = source.address;
    const cells = layers.reduce((count, layer) => count + layer.cells.length, 0);
    summary.textContent = `${across} × ${down} cells of ${source.tileWidth} × ${source.tileHeight} · ${counted(cells, "cell", "cells")} in ${counted(layers.length, "layer", "layers")}${hidden}${without}`;
    point = (line) => {
      const found = layers.flatMap((layer) => layer.cells).find((cell) => line >= cell[4] && line <= cell[5]);
      pointer.classList.toggle("pointed", Boolean(found));
      if (found) {
        const at = placeOf(found[0], found[1]);
        Object.assign(pointer.style, { left: `${at.left}px`, top: `${at.top}px`, width: `${source.tileWidth}px`, height: `${source.tileHeight}px` });
      }
    };
    whatIsAt = (event) => {
      const box = drawing.getBoundingClientRect();
      const x = bounds.minX + Math.floor(((event.clientX - box.left) / box.width) * across);
      const y = bounds.maxY - Math.floor(((event.clientY - box.top) / box.height) * down);
      const found = onTop.get(`${x},${y}`);
      if (event.target !== drawing && event.target !== pointer) {
        return undefined;
      }
      if (!found) {
        return { about: `cell ${x}, ${y} · empty`, line: 0 };
      }
      const done = [found.cell[3] & ACROSS ? "mirrored across" : "", found.cell[3] & UP_DOWN ? "mirrored up and down" : "", found.cell[3] & TURNED ? "turned" : ""].filter(Boolean);
      return { about: `cell ${x}, ${y} · tile ${found.cell[2]} · layer ${found.layer.id}${done.length ? ` · ${done.join(", ")}` : ""}`, line: found.cell[4] };
    };
  }

  function colour(name, otherwise) {
    return getComputedStyle(document.body).getPropertyValue(name).trim() || otherwise;
  }

  function stretched(context, picture, item) {
    const [fromX, fromY, wide, high] = item.from;
    const left = -item.width / 2;
    const top = -item.height / 2;
    if (!item.slice9) {
      context.drawImage(picture, fromX, fromY, wide, high, left, top, item.width, item.height);
      return;
    }
    const fitted = (first, second, room) => {
      const share = first + second > room && first + second > 0 ? room / (first + second) : 1;
      return [first * share, second * share];
    };
    const [keptLeft, keptTop, keptRight, keptBottom] = item.slice9;
    const [fromLeft, fromRight] = fitted(keptLeft, keptRight, wide);
    const [fromTop, fromBottom] = fitted(keptTop, keptBottom, high);
    const [toLeft, toRight] = fitted(keptLeft, keptRight, item.width);
    const [toTop, toBottom] = fitted(keptTop, keptBottom, item.height);
    const columns = [
      [fromX, fromLeft, left, toLeft],
      [fromX + fromLeft, wide - fromLeft - fromRight, left + toLeft, item.width - toLeft - toRight],
      [fromX + wide - fromRight, fromRight, left + item.width - toRight, toRight],
    ];
    const rows = [
      [fromY, fromTop, top, toTop],
      [fromY + fromTop, high - fromTop - fromBottom, top + toTop, item.height - toTop - toBottom],
      [fromY + high - fromBottom, fromBottom, top + item.height - toBottom, toBottom],
    ];
    rows.forEach(([sourceY, sourceHigh, y, tall]) => {
      columns.forEach(([sourceX, sourceWide, x, across]) => {
        if (sourceWide > 0 && sourceHigh > 0 && across > 0 && tall > 0) {
          context.drawImage(picture, sourceX, sourceY, sourceWide, sourceHigh, x, y, across, tall);
        }
      });
    });
  }

  function filled(context, item, colour) {
    const across = item.width / 2;
    const up = item.height / 2;
    context.fillStyle = colour;
    if (item.kind !== "pie") {
      context.fillRect(-across, -up, item.width, item.height);
      return;
    }
    const sweep = ((item.sweep ?? 360) * Math.PI) / 180;
    const isWhole = Math.abs(sweep) >= 2 * Math.PI;
    const inner = Math.max(0, Math.min(item.inner ?? 0, across));
    const innerUp = across > 0 ? (inner * up) / across : 0;
    const reach = item.square ? Math.hypot(across, up) : 0;
    context.save();
    context.beginPath();
    if (item.square) {
      context.rect(-across, -up, item.width, item.height);
      context.clip();
      context.beginPath();
    }
    if (isWhole) {
      context.ellipse(0, 0, reach || across, reach || up, 0, 0, 2 * Math.PI);
      if (inner > 0) {
        context.moveTo(inner, 0);
        context.ellipse(0, 0, inner, innerUp, 0, 0, 2 * Math.PI);
      }
    } else {
      context.ellipse(0, 0, reach || across, reach || up, 0, 0, -sweep, sweep > 0);
      if (inner > 0) {
        context.ellipse(0, 0, inner, innerUp, 0, -sweep, 0, sweep < 0);
      } else {
        context.lineTo(0, 0);
      }
      context.closePath();
    }
    context.fill("evenodd");
    context.restore();
  }

  function written(context, item, otherwise) {
    const lines = String(item.text ?? "").split("\n");
    const high = item.textSize || Math.max(1, Math.min(item.height * 0.6, 24));
    context.fillStyle = item.tint ?? otherwise;
    context.font = `${high}px sans-serif`;
    context.textAlign = item.leanX < 0 ? "left" : item.leanX > 0 ? "right" : "center";
    context.textBaseline = "middle";
    const x = ((item.leanX ?? 0) * item.width) / 2;
    const leading = high * 1.2;
    const whole = leading * lines.length;
    const top = item.leanY > 0 ? -item.height / 2 : item.leanY < 0 ? item.height / 2 - whole : -whole / 2;
    lines.forEach((line, index) => {
      const y = item.textSize ? top + leading * (index + 0.5) : leading * (index - (lines.length - 1) / 2);
      context.fillText(line, x, y, item.textSize ? undefined : item.width || undefined);
    });
  }

  function drawScene() {
    const items = shown.items;
    extent = { width: Math.max(1, shown.maxX - shown.minX), height: Math.max(1, shown.maxY - shown.minY) };
    scene = made("canvas", "scene");
    stage.appendChild(scene);
    const pictures = shown.images.map((image) => {
      const picture = new Image();
      picture.onload = () => redraw();
      picture.src = image.address;
      return picture;
    });
    const cellsOf = shown.maps.map((placed) => new Set(placed.cells.map((cell) => `${cell[0]},${cell[1]}`)));
    let hovered;
    let pointedLine = 0;
    let isAsked = false;
    const onScreen = (x, y) => ({ x: offset.x + zoom * (x - shown.minX), y: offset.y + zoom * (shown.maxY - y) });
    const isPointed = (item) => pointedLine >= item.line && pointedLine <= item.lastLine;

    const paint = () => {
      isAsked = false;
      const ratio = window.devicePixelRatio || 1;
      const wide = stage.clientWidth;
      const high = stage.clientHeight;
      if (scene.width !== Math.round(wide * ratio) || scene.height !== Math.round(high * ratio)) {
        scene.width = Math.round(wide * ratio);
        scene.height = Math.round(high * ratio);
      }
      const context = scene.getContext("2d");
      const plain = () => context.setTransform(ratio, 0, 0, ratio, 0, 0);
      const placed = (item) => {
        const [a, b, c, d, x, y] = item.across;
        const at = onScreen(x, y);
        context.setTransform(ratio * zoom * a, -ratio * zoom * b, -ratio * zoom * c, ratio * zoom * d, ratio * at.x, ratio * at.y);
      };
      const outline = colour("--vscode-editorIndentGuide-activeBackground", "#888");
      const shape = colour("--vscode-editorInfo-foreground", "#3794ff");
      const focus = colour("--vscode-focusBorder", "#3794ff");
      const text = colour("--vscode-editor-foreground", "#ccc");
      const faint = colour("--vscode-editorWidget-border", "#555");
      plain();
      context.clearRect(0, 0, wide, high);
      context.imageSmoothingEnabled = zoom < 1;
      const origin = onScreen(0, 0);
      context.strokeStyle = faint;
      context.lineWidth = 1;
      context.beginPath();
      context.moveTo(0, Math.round(origin.y) + 0.5);
      context.lineTo(wide, Math.round(origin.y) + 0.5);
      context.moveTo(Math.round(origin.x) + 0.5, 0);
      context.lineTo(Math.round(origin.x) + 0.5, high);
      context.stroke();
      items.forEach((item) => {
        const isMarked = item === hovered || isPointed(item);
        const traced = (stroke, width, dash) => {
          plain();
          context.strokeStyle = stroke;
          context.lineWidth = width;
          context.setLineDash(dash ?? []);
          context.stroke();
          context.setLineDash([]);
        };
        if (item.kind === "marker") {
          const at = onScreen(item.across[4], item.across[5]);
          plain();
          context.beginPath();
          context.moveTo(at.x, at.y - 5);
          context.lineTo(at.x + 5, at.y);
          context.lineTo(at.x, at.y + 5);
          context.lineTo(at.x - 5, at.y);
          context.closePath();
          context.fillStyle = isMarked ? focus : outline;
          context.fill();
          return;
        }
        placed(item);
        context.globalAlpha = item.off ? 0 : (item.alpha ?? 1);
        if (item.kind === "display") {
          context.globalAlpha = 1;
          context.beginPath();
          context.rect(-item.width / 2, -item.height / 2, item.width, item.height);
          traced(outline, 1);
          return;
        }
        if (item.kind === "sprite") {
          const picture = pictures[item.image];
          if (picture?.complete && picture.naturalWidth) {
            context.save();
            context.scale(item.flip & ACROSS ? -1 : 1, item.flip & UP_DOWN ? -1 : 1);
            stretched(context, picture, item);
            context.restore();
          }
        } else if (item.kind === "panel" || item.kind === "pie") {
          filled(context, item, item.tint ?? text);
        } else if (item.kind === "tilemap") {
          const picture = pictures[item.image];
          const { tileWidth, tileHeight, tiles, cells } = shown.maps[item.map];
          if (picture?.complete && picture.naturalWidth) {
            cells.forEach(([x, y, tile, done]) => {
              context.save();
              context.translate((x + 0.5) * tileWidth, -(y + 0.5) * tileHeight);
              const isTurned = Boolean(done & TURNED);
              if (isTurned) {
                context.rotate(Math.PI / 2);
              }
              context.scale(done & ACROSS ? -1 : 1, done & UP_DOWN ? -1 : 1);
              const across = isTurned ? tileHeight : tileWidth;
              const down = isTurned ? tileWidth : tileHeight;
              context.drawImage(picture, tiles[tile][0], tiles[tile][1], tileWidth, tileHeight, -across / 2, -down / 2, across, down);
              context.restore();
            });
          }
          if (!isMarked) {
            return;
          }
          context.beginPath();
          cells.forEach(([x, y]) => context.rect(x * tileWidth, -(y + 1) * tileHeight, tileWidth, tileHeight));
          traced(focus, 1);
          return;
        } else if (item.kind === "label") {
          written(context, item, text);
        }
        context.globalAlpha = 1;
        context.beginPath();
        if (item.kind === "sphere") {
          context.arc(0, 0, item.radius, 0, 2 * Math.PI);
        } else if (item.kind === "capsule") {
          context.roundRect(-item.width / 2, -item.height / 2, item.width, item.height, item.radius);
        } else {
          context.rect(-item.width / 2, -item.height / 2, item.width, item.height);
        }
        if (isMarked) {
          traced(focus, 2);
        } else if (item.kind === "label" || item.off) {
          traced(outline, 1, [3, 3]);
        } else if (item.kind === "panel" || item.kind === "pie") {
          traced(faint, 1);
        } else if (item.kind !== "sprite") {
          traced(shape, 1);
        }
      });
      plain();
    };
    redraw = () => {
      if (!isAsked) {
        isAsked = true;
        requestAnimationFrame(paint);
      }
    };

    const under = (event) => {
      const box = stage.getBoundingClientRect();
      const at = { x: event.clientX - box.left, y: event.clientY - box.top };
      const world = { x: shown.minX + (at.x - offset.x) / zoom, y: shown.maxY - (at.y - offset.y) / zoom };
      for (let index = items.length - 1; index >= 0; index--) {
        const item = items[index];
        const [a, b, c, d, x, y] = item.across;
        if (item.kind === "display") {
          continue;
        }
        if (item.kind === "marker") {
          const mark = onScreen(x, y);
          if (Math.hypot(mark.x - at.x, mark.y - at.y) <= 7) {
            return item;
          }
          continue;
        }
        const turn = a * d - b * c;
        if (!turn) {
          continue;
        }
        const across = (d * (world.x - x) - c * (world.y - y)) / turn;
        const up = (-b * (world.x - x) + a * (world.y - y)) / turn;
        if (item.kind === "tilemap") {
          const placed = shown.maps[item.map];
          if (cellsOf[item.map].has(`${Math.floor(across / placed.tileWidth)},${Math.floor(up / placed.tileHeight)}`)) {
            return item;
          }
        } else if (item.kind === "sphere" ? Math.hypot(across, up) <= item.radius : Math.abs(across) <= item.width / 2 && Math.abs(up) <= item.height / 2) {
          return item;
        }
      }
      return undefined;
    };
    whatIsAt = (event) => {
      const item = under(event);
      if (item !== hovered) {
        hovered = item;
        redraw();
      }
      if (!item) {
        return undefined;
      }
      const name = `${item.owner || shown.file}${item.component ? `#${item.component}` : ""}`;
      const size = item.kind === "marker" || item.kind === "tilemap" ? "" : ` · ${item.width} × ${item.height}`;
      const at = `at ${Number(item.across[4].toFixed(2))}, ${Number(item.across[5].toFixed(2))}`;
      if (shown.display) {
        const node = item.kind === "marker" ? `${item.type}, not drawn here` : item.kind === "sprite" ? `box of ${shown.images[item.image].path}` : item.kind === "label" ? `text “${item.text ?? ""}”` : item.type;
        const layer = item.layer ? ` · layer ${item.layer}` : "";
        return { about: `${name} · ${node}${size} · ${at}${layer}${item.off ? " · not shown in the game" : ""}`, line: item.line };
      }
      const what = item.kind === "marker" ? `${item.type}, not drawn here` : item.kind === "sprite" ? `sprite of ${shown.images[item.image].path}` : item.kind === "label" ? `label “${item.text ?? ""}”` : item.kind === "tilemap" ? "tile map" : `${item.kind} of a collision object`;
      return { about: `${name} · ${what}${size} · ${at} · depth ${Number(item.depth.toFixed(3))}`, line: item.line };
    };
    point = (line) => {
      pointedLine = line;
      redraw();
    };
    const marked = items.filter((item) => item.kind === "marker").length;
    const markedSaid = marked ? `, ${marked} marked and not drawn` : "";
    const cutShort = shown.cutShort ? " · too large to show whole: the rest is left out" : "";
    summary.textContent = shown.display
      ? `${counted(shown.objects, "node", "nodes")}${markedSaid} · display ${shown.display[0]} × ${shown.display[1]}${cutShort}`
      : `${counted(shown.objects, "game object", "game objects")} · ${counted(items.length - marked, "component", "components")} drawn${markedSaid} · ${Math.round(extent.width)} × ${Math.round(extent.height)}${cutShort}`;
  }

  function draw() {
    canvas.replaceChildren();
    scene?.remove();
    scene = undefined;
    point = () => {};
    whatIsAt = () => undefined;
    redraw = () => {};
    ({ tilesource: drawTileSource, tilemap: drawTileMap, scene: drawScene }[shown.kind] ?? drawAtlas)();
    point(pointed);
    if (isFitted) {
      fit();
    } else {
      apply();
    }
  }

  function say(text, isAFailure) {
    status.textContent = text ?? "";
    status.classList.toggle("failure", Boolean(isAFailure));
  }

  window.addEventListener("message", (event) => {
    const message = event.data;
    if (message.shown) {
      isFitted = isFitted || shown?.file !== message.shown.file || shown?.atlas !== message.shown.atlas;
      shown = message.shown;
      draw();
    }
    if (message.failure) {
      say(message.failure, true);
    } else if (message.status !== undefined) {
      say(message.status, false);
    }
    if (message.line) {
      pointed = message.line;
      point(pointed);
    }
  });

  let saying;
  function saySize() {
    clearTimeout(saying);
    saying = setTimeout(() => host.postMessage({ size: { width: window.innerWidth, height: window.innerHeight } }), PAUSE_BEFORE_SAYING_THE_SIZE);
  }

  window.addEventListener("resize", () => {
    if (isFitted) {
      fit();
    } else {
      apply();
    }
    saySize();
  });

  stage.addEventListener(
    "wheel",
    (event) => {
      if (!shown) {
        return;
      }
      event.preventDefault();
      const before = zoom;
      zoom = Math.max(SMALLEST_ZOOM, Math.min(LARGEST_ZOOM, zoom * (event.deltaY < 0 ? ZOOM_OF_A_NOTCH : 1 / ZOOM_OF_A_NOTCH)));
      const at = { x: event.clientX - stage.getBoundingClientRect().left, y: event.clientY - stage.getBoundingClientRect().top };
      offset = { x: at.x - ((at.x - offset.x) * zoom) / before, y: at.y - ((at.y - offset.y) * zoom) / before };
      isFitted = false;
      apply();
    },
    { passive: false },
  );

  let held;
  stage.addEventListener("pointerdown", (event) => {
    held = { x: event.clientX, y: event.clientY, from: { ...offset }, hasMoved: false, on: whatIsAt(event) };
    stage.setPointerCapture(event.pointerId);
  });
  stage.addEventListener("pointermove", (event) => {
    if (!held) {
      about.textContent = whatIsAt(event)?.about ?? "";
      return;
    }
    const moved = { x: event.clientX - held.x, y: event.clientY - held.y };
    if (!held.hasMoved && Math.hypot(moved.x, moved.y) < A_DRAG_STARTS_AFTER) {
      return;
    }
    held.hasMoved = true;
    stage.classList.add("dragged");
    offset = { x: held.from.x + moved.x, y: held.from.y + moved.y };
    isFitted = false;
    apply();
  });
  stage.addEventListener("pointerup", (event) => {
    stage.releasePointerCapture(event.pointerId);
    stage.classList.remove("dragged");
    if (held && !held.hasMoved && held.on?.line) {
      host.postMessage({ reveal: held.on.line });
    }
    held = undefined;
  });
  stage.addEventListener("pointerleave", () => {
    about.textContent = "";
  });
  stage.addEventListener("dblclick", fit);

  saySize();
})();
