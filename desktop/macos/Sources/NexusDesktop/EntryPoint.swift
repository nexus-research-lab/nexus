import AppKit
import Dispatch
import Darwin

@main
enum NexusDesktopMain {
  private static var signalSources: [DispatchSourceSignal] = []

  @MainActor
  static func main() {
    let app = NSApplication.shared
    let delegate = AppDelegate()
    app.delegate = delegate
    installTerminationSignalHandlers()
    app.run()
  }

  private static func installTerminationSignalHandlers() {
    for signalNumber in [SIGINT, SIGTERM] {
      Darwin.signal(signalNumber, SIG_IGN)
      let source = DispatchSource.makeSignalSource(signal: signalNumber, queue: .main)
      source.setEventHandler {
        Task { @MainActor in
          NSApp.terminate(nil)
        }
      }
      source.resume()
      signalSources.append(source)
    }
  }
}
