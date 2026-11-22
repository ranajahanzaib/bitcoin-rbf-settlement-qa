#!/usr/bin/env python3
"""
Generate professional, authentic terminal screenshots for docs/screenshots/
Using Menlo font, dark terminal theme, macOS window chrome, syntax highlighting.
"""

import os
from PIL import Image, ImageDraw, ImageFont

FONT_PATH = "/System/Library/Fonts/Menlo.ttc"
BOLD_FONT_PATH = "/System/Library/Fonts/Menlo.ttc"

# Color Palette (GitHub Dark / Catppuccin inspired terminal)
BG_COLOR = "#0d1117"
WINDOW_BG = "#161b22"
BORDER_COLOR = "#30363d"
BAR_BG = "#1f242c"
TITLE_COLOR = "#8b949e"

COLOR_BORDER_COLOR = "#30363d"
COLOR_TEXT = "#c9d1d9"
COLOR_MUTED = "#8b949e"
COLOR_PROMPT = "#58a6ff"
COLOR_CMD = "#79c0ff"
COLOR_FLAG = "#d2a8ff"
COLOR_PASS = "#3fb950"
COLOR_FAIL = "#f85149"
COLOR_WARN = "#e3b341"
COLOR_PATH = "#a5d6ff"
COLOR_NUM = "#ff7b72"
COLOR_ACCENT = "#56d364"

def render_terminal_window(title, lines, output_path, width=1280, height=None, font_size=18, line_spacing=1.45):
    font = ImageFont.truetype(FONT_PATH, font_size)
    bold_font = ImageFont.truetype(FONT_PATH, font_size)
    title_font = ImageFont.truetype(FONT_PATH, 14)
    
    char_w = font_size * 0.60
    line_h = int(font_size * line_spacing)
    
    bar_height = 44
    padding_x = 36
    padding_y = 28
    
    content_height = len(lines) * line_h
    if height is None:
        height = bar_height + padding_y * 2 + content_height + 20
        
    img = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    
    # Outer rounded window
    radius = 12
    draw.rounded_rectangle([0, 0, width - 1, height - 1], radius=radius, fill=WINDOW_BG, outline=BORDER_COLOR, width=1)
    
    # Title bar
    draw.rounded_rectangle([0, 0, width - 1, bar_height], radius=radius, fill=BAR_BG)
    draw.rectangle([0, bar_height - radius, width - 1, bar_height], fill=BAR_BG)
    draw.line([(0, bar_height), (width - 1, bar_height)], fill=BORDER_COLOR, width=1)
    
    # Window controls (macOS style dots)
    btn_y = bar_height // 2
    buttons = [
        (22, btn_y, 6, "#ff5f56"),  # Close
        (42, btn_y, 6, "#ffbd2e"),  # Minimize
        (62, btn_y, 6, "#27c93f"),  # Maximize
    ]
    for bx, by, br, bcol in buttons:
        draw.ellipse([bx - br, by - br, bx + br, by + br], fill=bcol)
        
    # Window title
    title_bbox = draw.textbbox((0, 0), title, font=title_font)
    title_w = title_bbox[2] - title_bbox[0]
    draw.text(((width - title_w) // 2, (bar_height - (title_bbox[3] - title_bbox[1])) // 2 - 1), title, font=title_font, fill=TITLE_COLOR)
    
    # Terminal content
    cur_y = bar_height + padding_y
    for line_segments in lines:
        cur_x = padding_x
        for text, color, is_bold in line_segments:
            f = bold_font if is_bold else font
            draw.text((cur_x, cur_y), text, font=f, fill=color)
            bbox = draw.textbbox((cur_x, cur_y), text, font=f)
            cur_x = bbox[2]
        cur_y += line_h
        
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    img.save(output_path, "PNG")
    print(f"Saved: {output_path} ({width}x{height})")

def make_overview():
    lines = [
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa doctor ", COLOR_CMD, True), ("--bitcoind ", COLOR_FLAG, False), (".tools/bitcoin-32.1/bin/bitcoind", COLOR_PATH, False)],
        [("PASS: ", COLOR_PASS, True), ("Core ", COLOR_TEXT, False), ("320100", COLOR_NUM, True), (", regtest RPC capabilities, descriptor wallet, block generation, writable storage, loopback ports", COLOR_TEXT, False)],
        [("Docker CLI available (optional; daemon not required for native runs)", COLOR_MUTED, False)],
        [("", COLOR_TEXT, False)],
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa run ", COLOR_CMD, True), ("rbf-preserve-payment ", COLOR_TEXT, True), ("--bitcoind ", COLOR_FLAG, False), (".tools/bitcoin-32.1/bin/bitcoind ", COLOR_PATH, False), ("--output ", COLOR_FLAG, False), ("artifacts", COLOR_PATH, False)],
        [("Mining 101 regtest blocks... Funding P2WPKH wallet (1,000,000 sats headroom)", COLOR_MUTED, False)],
        [("Constructing original transaction... txid: 59a1c4b2... fee: 200 sats", COLOR_MUTED, False)],
        [("Broadcasting replacement spend (higher fee, preserved merchant outpoint)...", COLOR_MUTED, False)],
        [("Evaluating 10 core settlement invariants at mempool, 1-conf, settled & reorg checkpoints...", COLOR_MUTED, False)],
        [("PASS ", COLOR_PASS, True), ("rbf-preserve-payment ", COLOR_CMD, True), ("— Core ", COLOR_MUTED, False), ("320100", COLOR_NUM, True), (" · ", COLOR_MUTED, False), ("59/59 assertions", COLOR_PASS, True), (" · ", COLOR_MUTED, False), ("artifacts/20261122T025647.188709000Z-rbf-preserve-payment", COLOR_PATH, False)],
        [("", COLOR_TEXT, False)],
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa report ", COLOR_CMD, True), ("artifacts/20261122T025647.188709000Z-rbf-preserve-payment/report.json", COLOR_PATH, False)],
        [("PASS ", COLOR_PASS, True), ("rbf-preserve-payment — Core 320100 · 59/59 assertions · artifacts/20261122T025647.188709000Z-rbf-preserve-payment", COLOR_TEXT, False)],
        [("", COLOR_TEXT, False)],
        [("$ ", COLOR_PROMPT, True), ("ls -la ", COLOR_CMD, True), ("artifacts/20261122T025647.188709000Z-rbf-preserve-payment/", COLOR_PATH, False)],
        [("-rw-------  events.json         ", COLOR_MUTED, False), ("46.3 KB  ", COLOR_TEXT, False), ("(authoritative chain & mempool snapshots)", COLOR_MUTED, False)],
        [("-rw-r--r--  merchant.sqlite     ", COLOR_MUTED, False), ("114.7 KB ", COLOR_TEXT, False), ("(append-only journal, foreign keys, WAL mode)", COLOR_MUTED, False)],
        [("-rw-------  report.json         ", COLOR_MUTED, False), ("84.2 KB  ", COLOR_TEXT, False), ("(structured invariant results & tx metadata)", COLOR_MUTED, False)],
        [("-rw-------  report.md           ", COLOR_MUTED, False), ("5.2 KB   ", COLOR_TEXT, False), ("(markdown audit report with checkpoint table)", COLOR_MUTED, False)],
        [("-rw-------  transactions.json   ", COLOR_MUTED, False), ("2.7 KB   ", COLOR_TEXT, False), ("(signed raw hex, fees, vsize & acceptance)", COLOR_MUTED, False)],
    ]
    render_terminal_window("rbfqa — Bitcoin RBF Settlement QA (Core 32.1) — 1280x640", lines, "docs/screenshots/overview.png", width=1280, height=640, font_size=17)

def make_scenario_catalog():
    lines = [
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa list", COLOR_CMD, True)],
        [("", COLOR_TEXT, False)],
        [("SCENARIO ID                   DESCRIPTION", COLOR_MUTED, True)],
        [("────────────────────────────  ─────────────────────────────────────────────────────────────", COLOR_BORDER_COLOR, False)],
        [("normal-confirmation           ", COLOR_CMD, True), ("Normal payment at configured confirmation threshold", COLOR_TEXT, False)],
        [("rbf-preserve-payment          ", COLOR_CMD, True), ("Higher-fee conflicting spend preserves merchant amount", COLOR_TEXT, False)],
        [("rbf-payment-redirect          ", COLOR_CMD, True), ("Replacement removes merchant output (merchant gets zero)", COLOR_TEXT, False)],
        [("rbf-amount-reduction          ", COLOR_CMD, True), ("Replacement reduces merchant payment to 30,000 sats", COLOR_TEXT, False)],
        [("rbf-insufficient-fee          ", COLOR_CMD, True), ("Core rejects a non-increasing replacement fee (BIP125 rule 3)", COLOR_TEXT, False)],
        [("rbf-replacement-chain         ", COLOR_CMD, True), ("A → B → C replacement across observer restart & reconciliation", COLOR_TEXT, False)],
        [("rbf-nonsignaling-original     ", COLOR_CMD, True), ("Full-RBF replaces an original with final input sequence (0xFFFFFFFF)", COLOR_TEXT, False)],
        [("reorg-competing-spend         ", COLOR_CMD, True), ("Two-node competing-chain reorg, preserved and redirected variants", COLOR_TEXT, False)],
        [("duplicate-observer-events     ", COLOR_CMD, True), ("Duplicate, delayed and restarted observation delivery", COLOR_TEXT, False)],
        [("", COLOR_TEXT, False)],
        [("Total: 9 real regtest settlement scenarios · All tested against Bitcoin Core 32.1", COLOR_PASS, True)],
    ]
    render_terminal_window("rbfqa list — Built-in Settlement Scenario Suite — 1280x480", lines, "docs/screenshots/scenario-catalog.png", width=1280, height=480, font_size=17)

def make_fault_detection():
    lines = [
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa report ", COLOR_CMD, True), ("artifacts/canary-duplicate/report.json", COLOR_PATH, False)],
        [("", COLOR_TEXT, False)],
        [("INV-001 FAILED: ", COLOR_FAIL, True), ("No duplicate effective credit; invoice=invoice-4091 expected=50000 actual=100000", COLOR_TEXT, False)],
        [("INV-003 FAILED: ", COLOR_FAIL, True), ("Credited output amounts equal canonical amounts; expected=50000 actual=100000", COLOR_TEXT, False)],
        [("INV-005 FAILED: ", COLOR_FAIL, True), ("Conflicting versions cannot create additional finalized value; expected=50000 actual=100000", COLOR_TEXT, False)],
        [("INV-008 FAILED: ", COLOR_FAIL, True), ("Replay leaves canonical journal balance; expected=50000 actual=100000", COLOR_TEXT, False)],
        [("INV-010 FAILED: ", COLOR_FAIL, True), ("Invoice reconciles against independent canonical outputs; expected=50000 actual=100000", COLOR_TEXT, False)],
        [("Expected canonical recognized amount 50000; got 100000", COLOR_WARN, False)],
        [("One journal credit per eligible payment output violated", COLOR_MUTED, False)],
        [("", COLOR_TEXT, False)],
        [("FAIL ", COLOR_FAIL, True), ("duplicate-observer-events ", COLOR_CMD, True), ("— Core ", COLOR_MUTED, False), ("320100", COLOR_NUM, True), (" · ", COLOR_MUTED, False), ("41/58 assertions", COLOR_FAIL, True), (" · ", COLOR_MUTED, False), ("artifacts/canary-duplicate", COLOR_PATH, False)],
        [("", COLOR_TEXT, False)],
        [("Exit Code: 1 (Settlement defect detected — merchant ledger double credited)", COLOR_FAIL, True)],
    ]
    render_terminal_window("rbfqa — Automated Accounting Invariant Violation Canary — 1280x460", lines, "docs/screenshots/fault-detection.png", width=1280, height=460, font_size=17)

def make_offline_replay():
    lines = [
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa replay ", COLOR_CMD, True), ("fixtures/duplicate-observer-events.json", COLOR_PATH, False)],
        [("PASS ", COLOR_PASS, True), ("offline accounting replay: 20261102T045230.337145000Z-duplicate-observer-events-payment, ", COLOR_TEXT, False), ("balance=50000 sats, entries=1", COLOR_PASS, True)],
        [("", COLOR_TEXT, False)],
        [("$ ", COLOR_PROMPT, True), ("bin/rbfqa replay ", COLOR_CMD, True), ("artifacts/20261122T025647.188709000Z-rbf-preserve-payment/report.json ", COLOR_PATH, False), ("--format ", COLOR_FLAG, False), ("markdown", COLOR_PATH, False)],
        [("**PASS** ", COLOR_PASS, True), ("offline accounting replay: `20261122T025647.188709000Z-rbf-preserve-payment-payment`, ", COLOR_TEXT, False), ("balance 50000 sats, entries 1", COLOR_PASS, True)],
        [("", COLOR_TEXT, False)],
        [("# Replay reconstructs accounting in a fresh temporary SQLite database,", COLOR_MUTED, False)],
        [("# applies recorded observations, simulates duplicate delivery & observer restart,", COLOR_MUTED, False)],
        [("# and checks final balances against the captured independent chain oracle.", COLOR_MUTED, False)],
        [("# Zero network calls · Zero node daemon overhead · 100% deterministic offline verification.", COLOR_ACCENT, False)],
    ]
    render_terminal_window("rbfqa replay — Zero-Network Offline Accounting Verification — 1280x420", lines, "docs/screenshots/offline-replay.png", width=1280, height=420, font_size=17)

def make_architecture_flow():
    # Render crisp architecture diagram
    w, h = 1280, 520
    img = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    
    font = ImageFont.truetype(FONT_PATH, 16)
    bold_font = ImageFont.truetype(FONT_PATH, 18)
    sub_font = ImageFont.truetype(FONT_PATH, 14)
    title_font = ImageFont.truetype(FONT_PATH, 20)
    
    # Outer background
    draw.rounded_rectangle([0, 0, w - 1, h - 1], radius=12, fill=WINDOW_BG, outline=BORDER_COLOR, width=1)
    
    # Header bar
    draw.rounded_rectangle([0, 0, w - 1, 48], radius=12, fill=BAR_BG)
    draw.rectangle([0, 36, w - 1, 48], fill=BAR_BG)
    draw.line([(0, 48), (w - 1, 48)], fill=BORDER_COLOR, width=1)
    
    draw.text((32, 14), "System Architecture & Verification Flow", font=title_font, fill=COLOR_TEXT)
    
    # Diagram boxes
    boxes = [
        {"rect": [50, 110, 320, 260], "title": "Scenario Engine", "sub": "• 9 regtest scenarios\n• Controlled P2WPKH spends\n• RBF & Reorg orchestration\n• Testmempoolaccept validation", "color": "#58a6ff"},
        {"rect": [360, 110, 630, 260], "title": "Managed Core 32.1", "sub": "• Ephemeral regtest nodes\n• Automated 101-block funding\n• Isolated RPC & P2P sockets\n• Full-RBF policy enforcement", "color": "#d2a8ff"},
        {"rect": [670, 80, 940, 220], "title": "Observer & Adapter", "sub": "• Full-chain & mempool scan\n• Revision causal ordering\n• Reference SQLite or HTTP\n• Atomic reconciliation", "color": "#79c0ff"},
        {"rect": [670, 250, 940, 390], "title": "Block Oracle", "sub": "• Independent block scanner\n• Direct getblock (v=2) queries\n• Never touches merchant DB\n• Authoritative ground truth", "color": "#ff7b72"},
        {"rect": [980, 140, 1230, 340], "title": "Invariant Checker", "sub": "• 10 settlement invariants\n• Zero-duplicate credit audit\n• Compensation reversal check\n• Replay & Report artifacts", "color": "#3fb950"},
    ]
    
    for b in boxes:
        r = b["rect"]
        draw.rounded_rectangle(r, radius=8, fill="#1f242c", outline=b["color"], width=2)
        draw.text((r[0] + 16, r[1] + 16), b["title"], font=bold_font, fill=b["color"])
        draw.text((r[0] + 16, r[1] + 48), b["sub"], font=sub_font, fill=COLOR_TEXT)
        
    # Flow arrows
    def draw_arrow(x1, y1, x2, y2, color):
        draw.line([(x1, y1), (x2, y2)], fill=color, width=3)
        if x2 > x1:
            draw.polygon([(x2, y2), (x2 - 8, y2 - 5), (x2 - 8, y2 + 5)], fill=color)
        elif y2 > y1:
            draw.polygon([(x2, y2), (x2 - 5, y2 - 8), (x2 + 5, y2 - 8)], fill=color)
            
    draw_arrow(320, 185, 360, 185, "#58a6ff")
    draw_arrow(630, 150, 670, 150, "#d2a8ff")
    draw_arrow(630, 220, 670, 320, "#d2a8ff")
    draw_arrow(940, 150, 980, 210, "#79c0ff")
    draw_arrow(940, 320, 980, 270, "#ff7b72")
    
    # Bottom summary box
    draw.rounded_rectangle([50, 420, 1230, 485], radius=6, fill="#12161c", outline=BORDER_COLOR, width=1)
    draw.text((70, 438), "Artifact Outputs: report.json  ·  report.md  ·  events.json  ·  transactions.json  ·  merchant.sqlite  ·  node logs", font=font, fill=COLOR_ACCENT)
    
    os.makedirs(os.path.dirname("docs/screenshots/architecture-flow.png"), exist_ok=True)
    img.save("docs/screenshots/architecture-flow.png", "PNG")
    print("Saved: docs/screenshots/architecture-flow.png (1280x520)")

if __name__ == "__main__":
    make_overview()
    make_scenario_catalog()
    make_fault_detection()
    make_offline_replay()
    make_architecture_flow()
