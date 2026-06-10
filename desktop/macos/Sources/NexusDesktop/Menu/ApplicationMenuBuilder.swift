import AppKit

@MainActor
enum ApplicationMenuBuilder {
  static func install(target: AppDelegate) {
    let mainMenu = NSMenu()
    NSApp.mainMenu = mainMenu

    mainMenu.addItem(appMenuItem(target: target))
    mainMenu.addItem(fileMenuItem())
    mainMenu.addItem(editMenuItem())
    mainMenu.addItem(viewMenuItem(target: target))
    mainMenu.addItem(windowMenuItem(target: target))
  }

  private static func appMenuItem(target: AppDelegate) -> NSMenuItem {
    let appName = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
      ?? Bundle.main.object(forInfoDictionaryKey: "CFBundleName") as? String
      ?? "Nexus"
    let item = NSMenuItem()
    let menu = NSMenu(title: appName)

    menu.addItem(menuItem("关于 \(appName)", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), target: NSApp))
    menu.addItem(menuItem("检查更新...", action: #selector(AppDelegate.checkForUpdates(_:)), target: target))
    menu.addItem(.separator())
    menu.addItem(menuItem("设置...", action: #selector(AppDelegate.showPreferences(_:)), key: ",", target: target))
    menu.addItem(.separator())

    let servicesItem = NSMenuItem(title: "服务", action: nil, keyEquivalent: "")
    let servicesMenu = NSMenu(title: "服务")
    servicesItem.submenu = servicesMenu
    NSApp.servicesMenu = servicesMenu
    menu.addItem(servicesItem)
    menu.addItem(.separator())

    menu.addItem(menuItem("隐藏 \(appName)", action: #selector(NSApplication.hide(_:)), key: "h", target: NSApp))
    menu.addItem(menuItem(
      "隐藏其他",
      action: #selector(NSApplication.hideOtherApplications(_:)),
      key: "h",
      modifiers: [.command, .option],
      target: NSApp
    ))
    menu.addItem(menuItem("全部显示", action: #selector(NSApplication.unhideAllApplications(_:)), target: NSApp))
    menu.addItem(.separator())
    menu.addItem(menuItem("退出 \(appName)", action: #selector(NSApplication.terminate(_:)), key: "q", target: NSApp))

    item.submenu = menu
    return item
  }

  private static func fileMenuItem() -> NSMenuItem {
    let item = NSMenuItem()
    let menu = NSMenu(title: "文件")
    menu.addItem(menuItem("关闭窗口", action: #selector(NSWindow.performClose(_:)), key: "w"))
    item.submenu = menu
    return item
  }

  private static func editMenuItem() -> NSMenuItem {
    let item = NSMenuItem()
    let menu = NSMenu(title: "编辑")
    menu.addItem(menuItem("撤销", action: Selector(("undo:")), key: "z"))
    menu.addItem(menuItem("重做", action: Selector(("redo:")), key: "Z"))
    menu.addItem(.separator())
    menu.addItem(menuItem("剪切", action: #selector(NSText.cut(_:)), key: "x"))
    menu.addItem(menuItem("复制", action: #selector(NSText.copy(_:)), key: "c"))
    menu.addItem(menuItem("粘贴", action: #selector(NSText.paste(_:)), key: "v"))
    menu.addItem(menuItem("删除", action: #selector(NSText.delete(_:))))
    menu.addItem(.separator())
    menu.addItem(menuItem("全选", action: #selector(NSText.selectAll(_:)), key: "a"))
    item.submenu = menu
    return item
  }

  private static func viewMenuItem(target: AppDelegate) -> NSMenuItem {
    let item = NSMenuItem()
    let menu = NSMenu(title: "显示")
    menu.addItem(menuItem("重新载入界面", action: #selector(AppDelegate.reloadMainWindow(_:)), key: "r", target: target))
    item.submenu = menu
    return item
  }

  private static func windowMenuItem(target: AppDelegate) -> NSMenuItem {
    let item = NSMenuItem()
    let menu = NSMenu(title: "窗口")
    menu.addItem(menuItem(
      "显示启动器",
      action: #selector(AppDelegate.showLauncher(_:)),
      target: target
    ))
    menu.addItem(.separator())
    menu.addItem(menuItem("最小化", action: #selector(NSWindow.miniaturize(_:)), key: "m"))
    menu.addItem(menuItem("缩放", action: #selector(NSWindow.zoom(_:))))
    menu.addItem(.separator())
    menu.addItem(menuItem("全部置前", action: #selector(NSApplication.arrangeInFront(_:)), target: NSApp))
    NSApp.windowsMenu = menu
    item.submenu = menu
    return item
  }

  private static func menuItem(
    _ title: String,
    action: Selector?,
    key: String = "",
    modifiers: NSEvent.ModifierFlags = [.command],
    target: AnyObject? = nil
  ) -> NSMenuItem {
    let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
    item.keyEquivalentModifierMask = key.isEmpty ? [] : modifiers
    item.target = target
    return item
  }
}
