#!/usr/bin/env python3
"""Extract the Ryoport Material Symbols as normalized SVG path data."""

import argparse
import json
from pathlib import Path

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

FONT = "/usr/share/fonts/TTF/MaterialSymbolsRounded[FILL,GRAD,opsz,wght].ttf"

NAMES = """Add:add
Remove:remove
ArrowBack:arrow_back
ArrowForward:arrow_forward
ArrowUpward:arrow_upward
ArrowDownward:arrow_downward
Check:check
CheckCircle:check_circle
ChevronLeft:chevron_left
ChevronRight:chevron_right
Close:close
CropSquare:crop_square
FilterNone:filter_none
Minimize:minimize
ContentCopy:content_copy
Delete:delete
Download:download
Edit:edit
EditNote:edit_note
Draft:draft
Error:error
ExpandMore:expand_more
ExpandLess:expand_less
Folder:folder
FolderOpen:folder_open
History:history
Info:info
Key:key
Keyboard:keyboard
KeyboardReturn:keyboard_return
Link:link
LinkOff:link_off
Lock:lock
LockOpen:lock_open
MoreVert:more_vert
MoreHoriz:more_horiz
Menu:menu
DragIndicator:drag_indicator
Notifications:notifications
OpenInNew:open_in_new
Refresh:refresh
Search:search
Settings:settings
Tune:tune
Upload:upload
Visibility:visibility
VisibilityOff:visibility_off
Warning:warning
Schedule:schedule
Help:help
Cancel:cancel
Save:save
Undo:undo
Sort:sort
FilterList:filter_list
Label:label
Star:star
Bookmark:bookmark
Circle:circle
DoneAll:done_all
Home:home
Update:update
NewReleases:new_releases
Logout:logout
DesktopWindows:desktop_windows
Computer:computer
Monitor:monitor
Memory:memory
DeveloperBoard:developer_board
HardDrive:hard_drive
Storage:storage
Speed:speed
Usb:usb
PlayArrow:play_arrow
Stop:stop
Pause:pause
PowerSettingsNew:power_settings_new
Power:power
RestartAlt:restart_alt
PhotoCamera:photo_camera
Album:album
Cloud:cloud
CloudOff:cloud_off
DeployedCode:deployed_code
RocketLaunch:rocket_launch
Inventory2:inventory_2
Layers:layers
SmartDisplay:smart_display
VideogameAsset:videogame_asset
InstallDesktop:install_desktop
Construction:construction
Timer:timer
Bolt:bolt
Dns:dns
Router:router
Lan:lan
SettingsEthernet:settings_ethernet
Public:public
Hub:hub
Terminal:terminal
SwapHoriz:swap_horiz
VpnKey:vpn_key
Shield:shield
NetworkCheck:network_check
Apps:apps
Dashboard:dashboard
QueryStats:query_stats
ShowChart:show_chart
Monitoring:monitoring
DataUsage:data_usage
Sync:sync
Verified:verified
Wifi:wifi
Cable:cable
ScreenShare:screen_share
Dock:dock
ReceiptLong:receipt_long
TaskAlt:task_alt
Pending:pending
HourglassEmpty:hourglass_empty
BugReport:bug_report
Palette:palette
Contrast:contrast
LightMode:light_mode
DarkMode:dark_mode
DensityMedium:density_medium
MotionPhotosOff:motion_photos_off
TextFields:text_fields
Adjust:adjust
AutoMode:auto_mode
GridView:grid_view
ViewList:view_list
PushPin:push_pin
SubdirectoryArrowRight:subdirectory_arrow_right
South:south
North:north
Anchor:anchor
Timeline:timeline"""

FILLED = """PlayArrowFill:play_arrow
StopFill:stop
PauseFill:pause
PushPinFill:push_pin
StarFill:star
CheckCircleFill:check_circle
ErrorFill:error
WarningFill:warning
InfoFill:info
BookmarkFill:bookmark
CircleFill:circle
AnchorFill:anchor
TimelineFill:timeline
SettingsFill:settings"""


def requested(block: str, fill: int):
    for line in block.splitlines():
        export, material = line.split(":", 1)
        yield export, material, fill


def ligatures(font: TTFont) -> dict[str, str]:
    reverse = {}
    for codepoint, glyph in font.getBestCmap().items():
        if codepoint < 128:
            reverse[glyph] = chr(codepoint)
    result = {}
    for lookup in font["GSUB"].table.LookupList.Lookup:
        for subtable in lookup.SubTable:
            kind = lookup.LookupType
            if kind == 7:
                kind = subtable.ExtensionLookupType
                subtable = subtable.ExtSubTable
            if kind != 4:
                continue
            for first, alternatives in subtable.ligatures.items():
                for ligature in alternatives:
                    glyphs = [first, *ligature.Component]
                    name = "".join(reverse.get(glyph, "") for glyph in glyphs)
                    if name:
                        result[name] = ligature.LigGlyph
    return result


def extract(font_path: str, entries):
    source = TTFont(font_path)
    glyph_for_name = ligatures(source)
    by_fill = {}
    output = []
    for export, material, fill in entries:
        if fill not in by_fill:
            by_fill[fill] = instantiateVariableFont(
                source,
                {"FILL": fill, "GRAD": 0, "opsz": 24, "wght": 400},
                inplace=False,
                optimize=True,
            )
        font = by_fill[fill]
        glyph_name = glyph_for_name.get(material)
        if glyph_name is None:
            raise SystemExit(f"Material Symbols has no liga glyph for {material!r}")
        glyph_set = font.getGlyphSet()
        if glyph_name not in glyph_set:
            raise SystemExit(f"Material Symbols has no glyph {glyph_name!r} for {material!r}")
        units = font["head"].unitsPerEm
        pen = SVGPathPen(glyph_set)
        transform = TransformPen(pen, (24 / units, 0, 0, -24 / units, 0, 24))
        glyph_set[glyph_name].draw(transform)
        path = pen.getCommands()
        if not path:
            raise SystemExit(f"Material Symbols glyph {material!r} has no outline")
        output.append({"export": export, "name": material, "fill": fill, "path": path})
    return output


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--font", default=FONT)
    parser.add_argument("--output", type=Path, default=Path(__file__).with_name("manifest.json"))
    args = parser.parse_args()
    entries = [*requested(NAMES, 0), *requested(FILLED, 1)]
    manifest = {"font": args.font, "axes": {"GRAD": 0, "opsz": 24, "wght": 400}, "icons": extract(args.font, entries)}
    args.output.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
