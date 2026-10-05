class Deployboard < Formula
  desc "Deployboard: launchd inventory + Prometheus metrics + Telegram alerts"
  # Fork home: github.com/francistse/deployboard
  homepage "https://github.com/francistse/deployboard"
  url "https://github.com/francistse/deployboard/archive/refs/heads/main.tar.gz"
  version "0.0.1"
  license "MIT"
  head "https://github.com/francistse/deployboard.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-X main.Version=#{version}"),
           "./cmd/deployboard"
  end

  service do
    run [opt_bin/"deployboard", "--config", etc/"deployboard/config.json"]
    run_type :immediate
    keep_alive true
    log_path var/"log/deployboard.log"
    error_log_path var/"log/deployboard.log"
    environment_variables PATH: std_service_path_env
  end

  def caveats
    <<~EOS
      To enable Telegram alerts, store the bot token in the macOS Keychain:
        security add-generic-password -U -s deployboard-telegram -a bot_token -w "$TOKEN" -T /usr/bin/security

      The config file is at:
        #{etc}/deployboard/config.json

      Edit it to set your chat_id and alert preferences, then reload:
        launchctl kickstart -k gui/$(id -u)/com.deployboard.agent
    EOS
  end

  test do
    assert_match "deployboard", shell_output("#{bin}/deployboard --version")
  end
end
