import Foundation
import CoreGraphics
let owner = Int(CommandLine.arguments[1])!
let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
let visible = windows.contains { item in
    guard let pid = item[kCGWindowOwnerPID as String] as? Int, pid == owner,
          let layer = item[kCGWindowLayer as String] as? Int, layer == 0,
          let bounds = item[kCGWindowBounds as String] as? [String: Any],
          let width = bounds["Width"] as? Double, let height = bounds["Height"] as? Double else { return false }
    return width > 300 && height > 200
}
exit(visible ? 0 : 1)
