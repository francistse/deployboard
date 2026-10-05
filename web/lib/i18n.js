/**
 * Dashboard UI strings. No bundler, no i18n library: four plain catalogs, a
 * Preact signal so a locale change re-renders any component that calls t(),
 * and localStorage key `deployboard:locale` (same prefix as the theme).
 *
 * t() never throws and never returns a blank string. Lookup order is the
 * active locale, then English, then the key itself.
 */
import { signal } from '@preact/signals';

export const LOCALE_KEY = 'deployboard:locale';

/** @type {readonly ['en', 'ja', 'zh-Hant', 'zh-Hans']} */
export const LOCALES = ['en', 'ja', 'zh-Hant', 'zh-Hans'];

/**
 * One row: key, en, ja, zh-Hant, zh-Hans.
 * [[token]] marks a fragment the UI wraps in <code>. {name} is interpolation.
 * Language names stay in their own script in every catalog.
 */
const ROWS = [
  ['access.readOnlyHint', 'Read-only mode — enable write mode in Settings → Access', '読み取り専用です。設定 → アクセスで書き込みモードを有効にしてください', '唯讀模式。請到「設定 → 存取」開啟寫入模式', '只读模式。请在「设置 → 访问」中开启写入模式'],
  ['ago.day', '{n} day ago', '{n}日前', '{n} 日前', '{n} 天前'],
  ['ago.days', '{n} days ago', '{n}日前', '{n} 日前', '{n} 天前'],
  ['ago.hour', '{n} hour ago', '{n}時間前', '{n} 小時前', '{n} 小时前'],
  ['ago.hours', '{n} hours ago', '{n}時間前', '{n} 小時前', '{n} 小时前'],
  ['ago.justNow', 'just now', 'たった今', '剛剛', '刚刚'],
  ['ago.min', '{n} min ago', '{n}分前', '{n} 分鐘前', '{n} 分钟前'],
  ['ago.month', '{n} month ago', '{n}か月前', '{n} 個月前', '{n} 个月前'],
  ['ago.months', '{n} months ago', '{n}か月前', '{n} 個月前', '{n} 个月前'],
  ['ago.year', '{n} year ago', '{n}年前', '{n} 年前', '{n} 年前'],
  ['ago.years', '{n} years ago', '{n}年前', '{n} 年前', '{n} 年前'],

  ['category.all', 'All', 'すべて', '全部', '全部'],
  ['category.noise', 'Noise', 'システム', '系統', '系统'],
  ['category.other', 'Other', '外部', '第三方', '第三方'],
  ['category.ours', 'Ours', '自分', '我的', '我的'],

  ['churn.many', '{n} restarts in the last {m}m', '過去{m}分で{n}回再起動', '過去 {m} 分鐘內重啟 {n} 次', '过去 {m} 分钟内重启 {n} 次'],
  ['churn.one', '{n} restart in the last {m}m', '過去{m}分で{n}回再起動', '過去 {m} 分鐘內重啟 {n} 次', '过去 {m} 分钟内重启 {n} 次'],

  ['classify.auto', 'Auto (by path)', '自動（パス）', '自動（依路徑）', '自动（按路径）'],
  ['classify.hide', 'Hide (vendor/OS)', '隠す（ベンダー / OS）', '隱藏（供應商 / 系統）', '隐藏（供应商 / 系统）'],
  ['classify.label', 'Classification:', '分類:', '分類：', '分类：'],
  ['classify.ours', 'Ours (pin)', '自分（固定）', '我的（固定）', '我的（固定）'],

  ['confirm.disable.note', 'launchctl disable + bootout — launchd will not start it again, including at login and after a reboot. The plist stays on disk; Enable undoes this. The dashboard then shows it as disabled instead of broken.', 'launchctl disable と bootout を実行します。ログイン時や再起動後も含め、launchd はもう起動しません。plist はディスクに残り、有効化で元に戻せます。ダッシュボードでは故障ではなく無効と表示されます。', '會執行 launchctl disable 與 bootout。包括登入時同重新開機後，launchd 都不會再啟動它。plist 仍留在磁碟，啟用就可以還原。儀表板其後會顯示為已停用，而不是故障。', '将执行 launchctl disable 与 bootout。包括登录时和重启之后，launchd 都不会再启动它。plist 仍留在磁盘上，启用即可还原。仪表板之后会把它显示为已禁用，而不是故障。'],
  ['confirm.disable.title', 'Retire this job?', 'このジョブを無効化しますか？', '停用這個工作？', '退役这个任务？'],
  ['confirm.disable.titleSelf', 'Retire the dashboard?', 'ダッシュボードを無効化しますか？', '停用儀表板？', '退役仪表板？'],
  ['confirm.enable.note', 'launchctl enable — clears the launchd disable flag. It does not start the job; press Start for that.', 'launchctl enable で launchd の無効フラグを外します。ジョブ自体は起動しないので、起動する場合は「開始」を押してください。', 'launchctl enable 會清除 launchd 的停用旗標。它不會啟動工作，要啟動請按「啟動」。', 'launchctl enable 会清除 launchd 的禁用标志。它不会启动任务；要启动请按「启动」。'],
  ['confirm.enable.title', 'Bring this job back?', 'このジョブを戻しますか？', '恢復這個工作？', '恢复这个任务？'],
  ['confirm.fallback', '{action} job?', '{action}しますか？', '{action}這個工作？', '{action}此任务？'],
  ['confirm.reload.note', 'launchctl bootout + bootstrap — the process is stopped and started again under launchd.', 'launchctl bootout と bootstrap を実行し、プロセスを止めてから launchd の下で起動し直します。', '執行 launchctl bootout 同 bootstrap，先停止行程，再由 launchd 重新啟動。', '执行 launchctl bootout 与 bootstrap：先停止进程，再在 launchd 下重新启动。'],
  ['confirm.reload.title', 'Reload job?', 'ジョブを再読み込みしますか？', '重新載入工作？', '重新加载任务？'],
  ['confirm.reload.titleSelf', 'Restart the dashboard?', 'ダッシュボードを再起動しますか？', '重啟儀表板？', '重启仪表板？'],
  ['confirm.self.reloadNote', 'this is the dashboard: bootout would delete its own job, so the process exits and launchd respawns it (KeepAlive). The page reconnects in a few seconds.', 'これはダッシュボード自身です。bootout すると自分のジョブが消えるため、プロセスが終了して launchd が起動し直します（KeepAlive）。数秒でページが再接続します。', '這是儀表板本身：bootout 會刪除它自己的工作，所以會由行程結束、launchd 重新啟動（KeepAlive）。幾秒後頁面就會重新連上。', '这是仪表板自身：bootout 会删掉它自己的任务，所以改为进程退出、由 launchd 重新拉起（KeepAlive）。页面几秒后重新连上。'],
  ['confirm.self.staysDown', 'this is the dashboard — nothing brings it back until launchctl bootstrap, or your next login; prefer Restart', 'これはダッシュボード自身です。launchctl bootstrap するか、次にログインするまで、誰も起動し直してくれません。再起動を使ってください', '這是儀表板本身。在你執行 launchctl bootstrap，或下次登入之前，沒有東西會把它叫回來；請改用重啟', '这是仪表板自身。在执行 launchctl bootstrap 或下次登录之前，没有东西会把它拉回来；请改用重启'],
  ['confirm.start.note', 'launchctl enable + kickstart (or bootstrap if it is not loaded) — it will run again under launchd.', 'launchctl enable と kickstart（未ロードなら bootstrap）を実行し、launchd の下で再び動かします。', '執行 launchctl enable 同 kickstart（未載入就用 bootstrap），讓它在 launchd 下再跑。', '执行 launchctl enable 与 kickstart（若未加载则 bootstrap），让它在 launchd 下再次运行。'],
  ['confirm.start.noteRetired', 'launchctl enable + bootstrap — clears the disable flag (this job was retired), then loads it.', 'launchctl enable と bootstrap を実行します。無効フラグを外して（このジョブは無効化されていました）からロードします。', '執行 launchctl enable 同 bootstrap。先清除停用旗標（這個工作已停用），再載入它。', '执行 launchctl enable 与 bootstrap。先清除禁用标志（此任务已退役），再加载它。'],
  ['confirm.start.title', 'Start job?', 'ジョブを開始しますか？', '啟動工作？', '启动任务？'],
  ['confirm.stop.loop', 'Stop this restart loop?', 'この再起動ループを止めますか？', '停止這個重啟循環？', '停止这个重启循环？'],
  ['confirm.stop.note', 'launchctl kill SIGTERM — the process stops and the job stays loaded, so launchd can start it again on schedule or at login.', 'launchctl kill SIGTERM を送ります。プロセスは止まり、ジョブはロードされたままなので、予定時刻やログイン時に launchd が再び起動できます。', '傳送 launchctl kill SIGTERM。行程會停止，工作仍保持載入，launchd 可以按排程或登入時再啟動。', '发送 launchctl kill SIGTERM。进程会停止，任务保持加载，launchd 仍可按计划或在登录时再次启动它。'],
  ['confirm.stop.noteKeepAlive', 'This job has KeepAlive, so sending a signal would just make launchd start it again. Stop unloads it instead: launchctl bootout. The plist file is not touched, the row stays visible as offline, and Start brings it back.', 'このジョブは KeepAlive です。シグナルを送ると launchd がすぐに起動し直します。停止は代わりにアンロードします（launchctl bootout）。plist は変更せず、行はオフラインのまま残り、開始で戻せます。', '這個工作有 KeepAlive，只送訊號的話 launchd 會立刻再啟動。停止會改為卸載它：launchctl bootout。plist 不會被改動，此列會保持離線顯示，啟動就可以把它叫回來。', '此任务启用了 KeepAlive，只发信号的话 launchd 会立刻再启动它。停止会改为卸载：launchctl bootout。plist 不会被修改，这一行会保持离线，启动即可把它拉回来。'],
  ['confirm.stop.title', 'Stop job?', 'ジョブを停止しますか？', '停止工作？', '停止任务？'],
  ['confirm.stop.titleSelf', 'Stop the dashboard?', 'ダッシュボードを停止しますか？', '停止儀表板？', '停止仪表板？'],
  ['confirm.stop.unload', 'Stop job? (it will be unloaded)', 'ジョブを停止しますか？（アンロードされます）', '停止工作？（會卸載）', '停止任务？（将卸载）'],

  ['diag.empty', 'No diagnostic checks available.', '実行できる診断項目がありません。', '沒有可用的診斷檢查。', '没有可用的诊断检查。'],
  ['diag.meaning.config', 'Configuration error (EX_CONFIG)', '設定エラー（EX_CONFIG）', '設定錯誤（EX_CONFIG）', '配置错误（EX_CONFIG）'],
  ['diag.meaning.general', 'General error', '一般的なエラー', '一般錯誤', '一般错误'],
  ['diag.meaning.io', 'I/O error', 'I/O エラー', 'I/O 錯誤', 'I/O 错误'],
  ['diag.meaning.killed', 'Killed by signal {n}', 'シグナル {n} で終了しました', '被訊號 {n} 終止', '被信号 {n} 终止'],
  ['diag.meaning.killedNamed', 'Killed by signal {n} ({name})', 'シグナル {n}（{name}）で終了しました', '被訊號 {n}（{name}）終止', '被信号 {n}（{name}）终止'],
  ['diag.meaning.normal', 'Normal exit', '正常終了', '正常結束', '正常退出'],
  ['diag.meaning.notExec', 'Command not executable', 'コマンドに実行権限がありません', '指令無法執行', '命令不可执行'],
  ['diag.meaning.notFound', 'Command not found', 'コマンドが見つかりません', '找不到指令', '找不到命令'],
  ['diag.meaning.permission', 'Permission denied', '権限がありません', '權限被拒', '权限被拒绝'],
  ['diag.meaning.unknown', 'Unknown exit code {n}', '不明な終了コード {n}', '未知的退出碼 {n}', '未知退出码 {n}'],
  ['diag.meaning.usage', 'Command usage error', 'コマンドの使い方エラー', '指令用法錯誤', '命令用法错误'],
  ['diag.msg.cannotStat', 'Cannot stat {path}: {err}', '{path} の情報を取得できません: {err}', '無法讀取 {path} 的狀態：{err}', '无法读取 {path} 的状态：{err}'],
  ['diag.msg.exit', 'Exit code {code}: {meaning}', '終了コード {code}: {meaning}', '退出碼 {code}：{meaning}', '退出码 {code}：{meaning}'],
  ['diag.msg.logDirsExist', 'Log parent directories exist', 'ログの親ディレクトリは存在します', '日誌的上層目錄存在', '日志的父目录存在'],
  ['diag.msg.logMissing', 'Log parent directory missing: {dirs}', 'ログの親ディレクトリがありません: {dirs}', '缺少日誌的上層目錄：{dirs}', '缺少日志的父目录：{dirs}'],
  ['diag.msg.noExec', '{path} lacks execute permission', '{path} に実行権限がありません', '{path} 沒有執行權限', '{path} 缺少执行权限'],
  ['diag.msg.noLogs', 'No log paths configured', 'ログのパスは設定されていません', '沒有設定日誌路徑', '未配置日志路径'],
  ['diag.msg.noOwnerPlatform', 'Cannot determine file owner on this platform', 'このプラットフォームでは所有者を判定できません', '在這個平台無法判斷檔案擁有者', '在此平台上无法判断文件所有者'],
  ['diag.msg.noPlist', 'No plist path known', 'plist のパスが分かりません', '未知 plist 路徑', '未知 plist 路径'],
  ['diag.msg.noProgram', 'No Program or ProgramArguments configured', 'Program も ProgramArguments も設定されていません', '沒有設定 Program 或 ProgramArguments', '未配置 Program 或 ProgramArguments'],
  ['diag.msg.noProgramPath', 'No program path to check', '確認できるプログラムのパスがありません', '沒有程式路徑可供檢查', '没有可检查的程序路径'],
  ['diag.msg.notExists', '{path} does not exist', '{path} は存在しません', '{path} 不存在', '{path} 不存在'],
  ['diag.msg.ownedByUid', '{path} owned by UID {uid}, not current user UID {current}', '{path} の所有者は UID {uid} で、現在のユーザー UID {current} ではありません', '{path} 的擁有者是 UID {uid}，不是目前使用者的 UID {current}', '{path} 的所有者是 UID {uid}，不是当前用户 UID {current}'],
  ['diag.msg.ownedByUser', '{path} owned by current user', '{path} の所有者は現在のユーザーです', '{path} 由目前使用者擁有', '{path} 由当前用户拥有'],
  ['diag.msg.pathExists', '{path} exists', '{path} は存在します', '{path} 存在', '{path} 存在'],
  ['diag.msg.permsBad', '{path} has permissions {perm} (group/world writable)', '{path} のパーミッションは {perm} です（グループまたは全員が書き込めます）', '{path} 的權限是 {perm}（群組或所有人可寫）', '{path} 的权限是 {perm}（组或其他用户可写）'],
  ['diag.msg.permsOk', '{path} permissions {perm} OK', '{path} のパーミッション {perm} は問題ありません', '{path} 的權限 {perm} 正常', '{path} 的权限 {perm} 正常'],
  ['diag.name.exit-code', 'Exit Code Analysis', '終了コード', '退出碼分析', '退出码分析'],
  ['diag.name.log-path-exists', 'Log Path', 'ログのパス', '日誌路徑', '日志路径'],
  ['diag.name.plist-owner', 'Plist Owner', 'plist の所有者', 'plist 擁有者', 'plist 所有者'],
  ['diag.name.plist-perms', 'Plist Permissions', 'plist のパーミッション', 'plist 權限', 'plist 权限'],
  ['diag.name.program-executable', 'Program Executable', 'プログラムの実行権限', '程式可執行', '程序可执行'],
  ['diag.name.program-exists', 'Program Path', 'プログラムのパス', '程式路徑', '程序路径'],
  ['diag.running', 'Running diagnostics…', '診断を実行しています…', '正在執行診斷…', '正在运行诊断…'],
  ['diag.sug.addExec', 'Add execute permission: chmod +x {path}', '実行権限を付けてください: chmod +x {path}', '請加上執行權限：chmod +x {path}', '请添加执行权限：chmod +x {path}'],
  ['diag.sug.addProgram', 'Add Program or ProgramArguments to the plist', 'plist に Program または ProgramArguments を追加してください', '請在 plist 加入 Program 或 ProgramArguments', '请在 plist 中添加 Program 或 ProgramArguments'],
  ['diag.sug.checkLogs', 'Check program logs for details', '詳しくはプログラムのログを確認してください', '請查看程式日誌了解詳情', '请查看程序日志了解详情'],
  ['diag.sug.checkPerms', 'Check executable and working directory permissions', '実行ファイルと作業ディレクトリの権限を確認してください', '請檢查執行檔同工作目錄的權限', '请检查可执行文件和工作目录的权限'],
  ['diag.sug.chmod', 'chmod +x {path}', 'chmod +x {path}', 'chmod +x {path}', 'chmod +x {path}'],
  ['diag.sug.chmod644', 'chmod 644 {path}', 'chmod 644 {path}', 'chmod 644 {path}', 'chmod 644 {path}'],
  ['diag.sug.chown', 'chown {uid} {path}', 'chown {uid} {path}', 'chown {uid} {path}', 'chown {uid} {path}'],
  ['diag.sug.io', 'Check plist encoding (UTF-8); ensure Full Disk Access for Terminal', 'plist の文字コード（UTF-8）を確認し、ターミナルにフルディスクアクセスがあるか見てください', '請檢查 plist 編碼（UTF-8），並確認「終端機」有完全取用磁碟的權限', '请检查 plist 编码（UTF-8），并确认“终端”拥有完全磁盘访问权限'],
  ['diag.sug.mkdir', 'mkdir -p {path}', 'mkdir -p {path}', 'mkdir -p {path}', 'mkdir -p {path}'],
  ['diag.sug.plutil', 'Validate plist: plutil -lint {path}', 'plist を検証してください: plutil -lint {path}', '請驗證 plist：plutil -lint {path}', '请校验 plist：plutil -lint {path}'],
  ['diag.sug.usage', 'Verify ProgramArguments in plist', 'plist の ProgramArguments を確認してください', '請核對 plist 內的 ProgramArguments', '请核对 plist 中的 ProgramArguments'],
  ['diag.sug.verifyPath', 'Verify the path in Program or ProgramArguments', 'Program または ProgramArguments のパスを確認してください', '請核對 Program 或 ProgramArguments 的路徑', '请核对 Program 或 ProgramArguments 中的路径'],
  ['diag.sug.verifyProgram', 'Verify Program/ProgramArguments path exists', 'Program / ProgramArguments のパスが存在するか確認してください', '請確認 Program / ProgramArguments 的路徑存在', '请确认 Program / ProgramArguments 的路径存在'],

  ['filter.noiseLockedTitle', 'Shows system & vendor jobs — click to reveal them', 'システムとベンダーのジョブです。クリックで表示します', '顯示系統同供應商的工作 — 點一下就會顯示', '显示系统与供应商的任务 — 点击即可显示'],
  ['filter.showNoise', 'Show system & vendor jobs', 'システムとベンダーのジョブを表示', '顯示系統與供應商的工作', '显示系统与供应商的任务'],

  ['group.jobCount', '{n} job(s)', '{n} 件', '{n} 個工作', '{n} 个任务'],
  ['group.nothing.disable', 'Nothing to disable in this group', 'このグループに無効化できるジョブはありません', '這個群組沒有可停用的工作', '此分组没有可禁用的任务'],
  ['group.nothing.reload', 'Nothing to reload in this group', 'このグループに再読み込みできるジョブはありません', '這個群組沒有可重新載入的工作', '此分组没有可重新加载的任务'],
  ['group.nothing.start', 'Nothing to start in this group', 'このグループに開始できるジョブはありません', '這個群組沒有可啟動的工作', '此分组没有可启动的任务'],
  ['group.nothing.stop', 'Nothing to stop in this group', 'このグループに停止できるジョブはありません', '這個群組沒有可停止的工作', '此分组没有可停止的任务'],
  ['group.note.disable', 'launchctl disable, so launchd will not start them again — including at login and after a reboot. The plists stay on disk and Enable undoes it. Targets: {labels}.', 'launchctl disable を実行します。ログイン時や再起動後も含め、launchd は再起動しません。plist は残り、有効化で元に戻せます。対象: {labels}。', '執行 launchctl disable，launchd 不會再啟動它們，包括登入時同重新開機後。plist 仍留在磁碟，啟用就可以還原。對象：{labels}。', '执行 launchctl disable，launchd 不会再启动它们，包括登录时和重启之后。plist 仍留在磁盘上，启用即可还原。目标：{labels}。'],
  ['group.note.reload', 'launchctl bootout + bootstrap, one after another, for: {labels}.', '次のジョブに対して、順番に launchctl bootout と bootstrap を実行します: {labels}。', '會逐一對以下工作執行 launchctl bootout 同 bootstrap：{labels}。', '将依次对以下任务执行 launchctl bootout 与 bootstrap：{labels}。'],
  ['group.note.start', 'Jobs that are already running are left alone. Targets: {labels}.', 'すでに動いているジョブはそのままです。対象: {labels}。', '已經在跑的工作不會動到。對象：{labels}。', '已经在运行的任务不会被动到。目标：{labels}。'],
  ['group.note.stop', 'KeepAlive jobs are unloaded (a signal would only restart them); the rest get SIGTERM. They stay listed as offline until you start them again. Targets: {labels}.', 'KeepAlive のジョブはアンロードします（シグナルでは再起動するだけです）。それ以外には SIGTERM を送ります。再び開始するまでオフラインと表示されます。対象: {labels}。', 'KeepAlive 的工作會被卸載（只送訊號的話只會再啟動）；其餘送 SIGTERM。再次啟動之前會保持離線。對象：{labels}。', 'KeepAlive 任务会被卸载（只发信号只会让它再启动）；其余发送 SIGTERM。再次启动之前会保持离线。目标：{labels}。'],
  ['group.restartAll', 'Restart all', 'すべて再起動', '全部重啟', '全部重启'],
  ['group.retireAll', 'Retire all', 'すべて無効化', '全部停用', '全部退役'],
  ['group.startAll', 'Start all', 'すべて開始', '全部啟動', '全部启动'],
  ['group.stopAll', 'Stop all', 'すべて停止', '全部停止', '全部停止'],
  ['group.title.disable', 'Retire every job in {group}?', '{group} のすべてのジョブを無効化しますか？', '停用 {group} 內的所有工作？', '退役 {group} 中的全部任务？'],
  ['group.title.reload', 'Restart every running job in {group}?', '{group} で実行中のジョブをすべて再起動しますか？', '重啟 {group} 內所有運行中的工作？', '重启 {group} 中所有正在运行的任务？'],
  ['group.title.start', 'Start every stopped job in {group}?', '{group} で止まっているジョブをすべて開始しますか？', '啟動 {group} 內所有已停止的工作？', '启动 {group} 中所有已停止的任务？'],
  ['group.title.stop', 'Stop every running job in {group}?', '{group} で実行中のジョブをすべて停止しますか？', '停止 {group} 內所有運行中的工作？', '停止 {group} 中所有正在运行的任务？'],
  ['group.toast', '{action} on {group}: {ok} done', '{group} に {action}: {ok} 件完了', '對 {group} 執行 {action}：完成 {ok} 個', '对 {group} 执行 {action}：完成 {ok} 个'],
  ['group.toastFail', '{action} on {group} failed: {message}', '{group} の {action} に失敗しました: {message}', '對 {group} 執行 {action} 失敗：{message}', '对 {group} 执行 {action} 失败：{message}'],
  ['group.toastFailed', ', {n} failed', '、{n} 件失敗', '，失敗 {n} 個', '，失败 {n} 个'],
  ['group.toastSkipped', ', {n} skipped', '、{n} 件スキップ', '，略過 {n} 個', '，跳过 {n} 个'],

  ['header.alertsCount', '{on} on · {off} off', '{on} オン · {off} オフ', '{on} 開 · {off} 關', '{on} 开 · {off} 关'],
  ['header.inventoryCount', '{ours} ours · {other} other · {noise} noise', '{ours} 自分 · {other} 外部 · {noise} システム', '{ours} 我的 · {other} 第三方 · {noise} 系統', '{ours} 我的 · {other} 第三方 · {noise} 系统'],
  ['header.inventoryTitle', 'Inventory', 'インベントリ', '清單', '清单'],
  ['header.readOnly', 'read-only', '読み取り専用', '唯讀', '只读'],
  ['header.readOnlyLocked', 'read-only (locked)', '読み取り専用（ロック）', '唯讀（已鎖定）', '只读（已锁定）'],
  ['header.readOnlyTitle', 'Monitoring only — enable write mode in Settings → Access to start/stop/reload', '監視のみです。開始・停止・再読み込みには、設定 → アクセスで書き込みモードを有効にしてください', '只供監察 — 要啟動、停止或重新載入，請到「設定 → 存取」開啟寫入模式', '仅监控 — 要启动、停止或重新加载，请在「设置 → 访问」中开启写入模式'],
  ['header.subtitle', 'macOS launchd inventory — your deployments only', 'macOS launchd の一覧 — 自分のデプロイだけ', 'macOS launchd 清單 — 只顯示你的部署', 'macOS launchd 清单 — 仅显示你的部署'],
  ['header.telegramTitle', 'Telegram alerts', 'Telegram 通知', 'Telegram 通知', 'Telegram 通知'],
  ['header.writeMode', 'write mode', '書き込みモード', '寫入模式', '写入模式'],
  ['header.writeTitle', 'start/stop/reload are allowed', '開始・停止・再読み込みができます', '可以啟動、停止同重新載入', '允许启动、停止和重新加载'],

  ['lang.en', 'English', 'English', 'English', 'English'],
  ['lang.ja', '日本語', '日本語', '日本語', '日本語'],
  ['lang.zh-Hans', '简体中文', '简体中文', '简体中文', '简体中文'],
  ['lang.zh-Hant', '繁體中文', '繁體中文', '繁體中文', '繁體中文'],

  ['logs.empty', '(empty)', '（空）', '（空白）', '（空）'],
  ['logs.filter', 'Filter logs…', 'ログを絞り込む…', '篩選日誌…', '筛选日志…'],
  ['logs.loadMore', 'Load more (currently {n} lines)', 'さらに読み込む（現在 {n} 行）', '載入更多（目前 {n} 行）', '加载更多（当前 {n} 行）'],
  ['logs.loading', 'Loading logs…', 'ログを読み込んでいます…', '正在載入日誌…', '正在加载日志…'],
  ['logs.loadingMore', 'Loading…', '読み込み中…', '載入中…', '加载中…'],
  ['logs.noPaths', 'No log paths configured in plist', 'plist にログのパスがありません', 'plist 沒有設定日誌路徑', 'plist 中未配置日志路径'],
  ['logs.showingAll', 'Showing all {n} lines', '全 {n} 行を表示しています', '已顯示全部 {n} 行', '已显示全部 {n} 行'],

  ['row.alertsDisable', 'Disable alerts', '通知を無効にする', '關閉通知', '关闭通知'],
  ['row.alertsEnable', 'Enable alerts', '通知を有効にする', '開啟通知', '开启通知'],
  ['row.churn', 'churn', '再起動', '反覆重啟', '反复重启'],
  ['row.disableTitle', 'Retire: launchd will not start it again, including at login', '無効化: ログイン時も含め、launchd は再起動しません', '停用：包括登入時，launchd 都不會再啟動它', '退役：包括登录时，launchd 也不会再启动它'],
  ['row.enableTitle', 'Clear the launchd disable flag so this can run again', 'launchd の無効フラグを外して、再び動かせるようにします', '清除 launchd 的停用旗標，讓它可以再跑', '清除 launchd 的禁用标志，使它可以再次运行'],
  ['row.keepAliveMark', 'KeepAlive: launchd starts this job again whenever it exits, so Stop unloads it instead of signalling', 'KeepAlive: 終了するたびに launchd が起動し直すため、停止はシグナルではなくアンロードします', 'KeepAlive：行程一結束 launchd 就會再啟動，所以停止會卸載它，而不是只送訊號', 'KeepAlive：进程一退出 launchd 就会再启动它，因此停止会卸载它，而不是只发信号'],
  ['row.keepAliveRuns', 'KeepAlive: launchd restarts this job whenever it exits', 'KeepAlive: 終了するたびに launchd が再起動します', 'KeepAlive：行程一結束 launchd 就會再啟動', 'KeepAlive：进程一退出 launchd 就会重启它'],
  ['row.pinnedHidden', '{label} pinned as hidden', '{label} を非表示に固定しました', '已將 {label} 固定為隱藏', '已将 {label} 固定为隐藏'],
  ['row.pinnedOurs', '{label} pinned as Ours', '{label} を「自分」に固定しました', '已將 {label} 固定為我的', '已将 {label} 固定为我的'],
  ['row.readOnlyLocked', 'Read-only — locked by the --read-only start-up flag', '読み取り専用 — --read-only 起動フラグでロックされています', '唯讀 — 被 --read-only 啟動參數鎖定', '只读 — 被 --read-only 启动参数锁定'],
  ['row.restartLoop', 'restart loop', '再起動ループ', '重啟循環', '重启循环'],
  ['row.restartNow', 'Restart it now (bootout + bootstrap)', '今すぐ再起動（bootout + bootstrap）', '立即重啟（bootout + bootstrap）', '立即重启（bootout + bootstrap）'],
  ['row.restartsAria', 'restarts: {summary}', '再起動: {summary}', '重啟：{summary}', '重启：{summary}'],
  ['row.restartsNone', 'none observed', '観測なし', '未觀察到', '未观察到'],
  ['row.restartsPerMinute', 'restarts per minute', '1分あたりの再起動', '每分鐘重啟次數', '每分钟重启次数'],
  ['row.retired', 'retired {ago}', '{ago}に無効化', '{ago}停用', '{ago}退役'],
  ['row.retiredTitle', 'Retired from the dashboard: launchd will not start it again ({ago}). Enable to bring it back.', 'ダッシュボードから無効化しました。launchd は再起動しません（{ago}）。有効化で戻せます。', '已在儀表板停用：launchd 不會再啟動它（{ago}）。啟用就可以恢復。', '已在仪表板退役：launchd 不会再启动它（{ago}）。启用即可恢复。'],
  ['row.runsSince', 'restarts since launchd loaded it', 'launchd がロードしてからの再起動回数', '自 launchd 載入以來的重啟次數', '自 launchd 加载以来的重启次数'],
  ['row.runningFor', 'running for {uptime}', '稼働時間 {uptime}', '已運行 {uptime}', '已运行 {uptime}'],
  ['row.self', 'self', '自身', '本身', '自身'],
  ['row.selfTitle', 'this is the Deployboard dashboard itself', 'これは Deployboard ダッシュボード自身です', '這是 Deployboard 儀表板本身', '这是 Deployboard 仪表板自身'],
  ['row.sendTest', 'Send test alert', 'テスト通知を送る', '傳送測試通知', '发送测试通知'],
  ['row.sourceHow', 'How this classification was decided', 'この分類の根拠', '這個分類是怎樣決定的', '此分类是如何判定的'],
  ['row.startDisabled', 'Clear the disable flag and load it (enable + bootstrap)', '無効フラグを外してロードします（enable + bootstrap）', '清除停用旗標並載入（enable + bootstrap）', '清除禁用标志并加载（enable + bootstrap）'],
  ['row.startNow', 'Load it now (enable + kickstart or bootstrap)', '今すぐロード（enable + kickstart または bootstrap）', '立即載入（enable + kickstart 或 bootstrap）', '立即加载（enable + kickstart 或 bootstrap）'],
  ['row.startScheduled', 'Load it now — launchd would otherwise run it at {when}', '今すぐロードします。しない場合、launchd は {when} に実行します', '立即載入 — 否則 launchd 會在 {when} 執行', '立即加载 — 否则 launchd 会在 {when} 运行'],
  ['row.stopSignal', 'Send SIGTERM, leaving the job loaded', 'SIGTERM を送り、ジョブはロードしたままにします', '傳送 SIGTERM，工作保持載入', '发送 SIGTERM，任务保持加载'],
  ['row.stopUnload', 'Unload it — a signal alone would make launchd restart it', 'アンロードします。シグナルだけでは launchd が再起動します', '卸載它 — 只送訊號的話 launchd 會再啟動', '卸载它 — 只发信号的话 launchd 会再启动它'],

  ['search.placeholder', 'Filter by label…', 'ラベルで絞り込む…', '以標籤篩選…', '按标签筛选…'],

  ['settings.access', 'Access', 'アクセス', '存取', '访问'],
  ['settings.accessHelp', 'Read-only mode refuses every [[reload]] / [[start]] / [[stop]] server-side, so nothing on this machine can be restarted by the dashboard. Turning it off writes [[read_only]] to [[config.json]], which is picked up within ~2 seconds — no restart.', '読み取り専用では、ダッシュボードからこの Mac のサービスを再起動できないよう、サーバー側で [[reload]] / [[start]] / [[stop]] をすべて拒否します。オフにすると [[config.json]] の [[read_only]] が書き換わり、再起動なしで約2秒以内に反映されます。', '唯讀模式會在伺服器端拒絕所有 [[reload]] / [[start]] / [[stop]]，所以儀表板無法重啟這部 Mac 上的任何服務。關掉它會把 [[read_only]] 寫入 [[config.json]]，大約 2 秒內生效，不用重新啟動。', '只读模式会在服务端拒绝每一次 [[reload]] / [[start]] / [[stop]]，因此仪表板无法重启这台 Mac 上的任何服务。关闭它会把 [[read_only]] 写入 [[config.json]]，大约 2 秒内生效，无需重启。'],
  ['settings.allow', 'Allow start / stop / reload', '開始 / 停止 / 再読み込みを許可', '允許啟動 / 停止 / 重新載入', '允许启动 / 停止 / 重新加载'],
  ['settings.aria', 'Settings', '設定', '設定', '设置'],
  ['settings.botToken', 'Bot token', 'ボットトークン', '機械人權杖', '机器人令牌'],
  ['settings.botTokenKeep', 'Bot token (leave blank to keep the current one)', 'ボットトークン（空欄のままなら現在のトークンを維持）', '機械人權杖（留空就保留現有的）', '机器人令牌（留空则保留当前令牌）'],
  ['settings.chatId', 'Chat id', 'チャット ID', '聊天 ID', '聊天 ID'],
  ['settings.checking', 'Checking…', '確認しています…', '檢查中…', '正在检查…'],
  ['settings.dataSources', 'Data sources', 'データソース', '資料來源', '数据来源'],
  ['settings.enableWriteLabel', 'start / stop / reload on every job', 'すべてのジョブで開始 / 停止 / 再読み込み', '對每個工作執行啟動 / 停止 / 重新載入', '对每个任务执行启动 / 停止 / 重新加载'],
  ['settings.enableWriteNote', 'The dashboard will be able to restart or kill services on this Mac. Read-only mode refuses those calls server-side; the switch writes read_only to config.json.', 'このダッシュボードから、この Mac のサービスを再起動または停止できるようになります。読み取り専用ではサーバー側でそれらの呼び出しを拒否します。スイッチは config.json の read_only を書き換えます。', '儀表板將可以重啟或結束這部 Mac 上的服務。唯讀模式會在伺服器端拒絕這些呼叫；這個開關會把 read_only 寫入 config.json。', '仪表板将能够重启或结束这台 Mac 上的服务。只读模式会在服务端拒绝这些调用；此开关会把 read_only 写入 config.json。'],
  ['settings.enableWriteTitle', 'Enable write mode?', '書き込みモードを有効にしますか？', '開啟寫入模式？', '开启写入模式？'],
  ['settings.forget', 'Forget token', 'トークンを削除', '刪除權杖', '删除令牌'],
  ['settings.keychainAccount', 'Keychain account', 'キーチェーンのアカウント', '鑰匙圈帳戶', '钥匙串账户'],
  ['settings.keychainService', 'Keychain service', 'キーチェーンのサービス', '鑰匙圈服務', '钥匙串服务'],
  ['settings.language', 'Language', '言語', '語言', '语言'],
  ['settings.neverEdits', 'The dashboard never edits a plist. It can only ask [[launchctl]] to start, stop or reload a label — and only when write mode is on.', 'ダッシュボードが plist を編集することはありません。書き込みモードがオンのときだけ、[[launchctl]] にラベルの開始・停止・再読み込みを依頼できます。', '儀表板永遠不會修改 plist。只有寫入模式開啟時，才會請 [[launchctl]] 啟動、停止或重新載入某個標籤。', '仪表板永远不会修改 plist。只有在写入模式开启时，才会让 [[launchctl]] 启动、停止或重新加载某个标签。'],
  ['settings.noToken', 'No token stored — alerts are disabled', 'トークンは保存されていません。通知は無効です', '未儲存權杖 — 通知已停用', '未保存令牌 — 通知已禁用'],
  ['settings.placeholderStored', '••••••••  stored', '••••••••  保存済み', '••••••••  已儲存', '••••••••  已保存'],
  ['settings.preferCli', 'Prefer the CLI?', 'CLI を使いますか？', '想用 CLI？', '更想用命令行？'],
  ['settings.readOnlyLocked', 'Read-only — locked by start-up flag', '読み取り専用 — 起動時フラグでロックされています', '唯讀 — 被啟動參數鎖定', '只读 — 被启动参数锁定'],
  ['settings.readOnlyRefused', 'Read-only — actions refused', '読み取り専用 — 操作は拒否されます', '唯讀 — 操作已被拒絕', '只读 — 操作已被拒绝'],
  ['settings.sendTest', 'Send test', 'テストを送信', '傳送測試', '发送测试'],
  ['settings.sourceConfig', 'setting source: config.json', '設定の出所: config.json', '設定來源：config.json', '设置来源：config.json'],
  ['settings.sourceFlag', 'setting source: --read-only', '設定の出所: --read-only', '設定來源：--read-only', '设置来源：--read-only'],
  ['settings.src.disabled', '[[launchctl print-disabled]] — intended-off jobs', '[[launchctl print-disabled]] — 意図して止めてあるジョブ', '[[launchctl print-disabled]] — 刻意關閉的工作', '[[launchctl print-disabled]] — 有意关闭的任务'],
  ['settings.src.list', '[[launchctl list]] — loaded jobs', '[[launchctl list]] — ロード済みのジョブ', '[[launchctl list]] — 已載入的工作', '[[launchctl list]] — 已加载的任务'],
  ['settings.src.plist', 'plist files in [[~/Library/LaunchAgents]] — schedule, paths, classification', '[[~/Library/LaunchAgents]] の plist — スケジュール、パス、分類', '[[~/Library/LaunchAgents]] 內的 plist — 排程、路徑、分類', '[[~/Library/LaunchAgents]] 中的 plist — 计划、路径、分类'],
  ['settings.src.print', '[[launchctl print]] — [[runs]] (restart counter), pid, last exit code', '[[launchctl print]] — [[runs]]（再起動回数）、pid、直前の終了コード', '[[launchctl print]] — [[runs]]（重啟次數）、pid、上次退出碼', '[[launchctl print]] — [[runs]]（重启次数）、pid、上次退出码'],
  ['settings.telegram', 'Telegram alerts', 'Telegram 通知', 'Telegram 通知', 'Telegram 通知'],
  ['settings.telegramHelp', 'The bot token is written straight to the macOS Keychain by this local server. It is never stored in a file, never logged, and never sent back to this page — the reader is pinned to [[/usr/bin/security]] so it keeps working across upgrades.', 'ボットトークンはこのローカルサーバーから macOS のキーチェーンへ直接書き込みます。ファイルには保存せず、ログにも残さず、このページへも返しません。読み取りは [[/usr/bin/security]] に固定してあるので、アップグレード後も動きます。', '機械人權杖由這部本地伺服器直接寫入 macOS 鑰匙圈。它不會存成檔案、不會寫入日誌，也不會送回這個頁面。讀取固定使用 [[/usr/bin/security]]，所以升級之後仍然有效。', '机器人令牌由这台本地服务器直接写入 macOS 钥匙串。它不会存成文件、不会写入日志，也不会发回本页面。读取固定为 [[/usr/bin/security]]，因此升级之后仍然有效。'],
  ['settings.title', 'Settings', '設定', '設定', '设置'],
  ['settings.toast.chatUpdated', 'Chat id updated', 'チャット ID を更新しました', '已更新聊天 ID', '已更新聊天 ID'],
  ['settings.toast.testSent', 'Test alert sent', 'テスト通知を送信しました', '已傳送測試通知', '已发送测试通知'],
  ['settings.toast.tokenRemoved', 'Token removed from the Keychain', 'キーチェーンからトークンを削除しました', '已從鑰匙圈刪除權杖', '已从钥匙串删除令牌'],
  ['settings.toast.tokenStored', 'Token stored in the macOS Keychain', 'トークンを macOS キーチェーンに保存しました', '權杖已存入 macOS 鑰匙圈', '令牌已存入 macOS 钥匙串'],
  ['settings.toast.writeOff', 'Write mode off — actions are refused again', '書き込みモードをオフにしました。操作は再び拒否されます', '已關閉寫入模式 — 操作會再被拒絕', '已关闭写入模式 — 操作将再次被拒绝'],
  ['settings.toast.writeOn', 'Write mode on — start/stop/reload allowed', '書き込みモードをオンにしました。開始・停止・再読み込みができます', '已開啟寫入模式 — 可以啟動、停止同重新載入', '已开启写入模式 — 允许启动、停止和重新加载'],
  ['settings.tokenActive', 'Token stored · alerts active', 'トークン保存済み · 通知は有効', '已儲存權杖 · 通知生效中', '已保存令牌 · 通知已启用'],
  ['settings.tokenInactive', 'Token stored · alerts inactive', 'トークン保存済み · 通知は無効', '已儲存權杖 · 通知未生效', '已保存令牌 · 通知未启用'],
  ['settings.unlock', 'To unlock: quit the LaunchAgent, drop [[--read-only]] from [[~/.config/deployboard/config.json]] or the agent\'s [[ProgramArguments]], and start it again. Deliberately not a UI switch — a monitoring box should not be talkable into killing a service.', '解除するには、LaunchAgent を終了し、[[~/.config/deployboard/config.json]] またはエージェントの [[ProgramArguments]] から [[--read-only]] を外して、起動し直してください。意図的に UI のスイッチにはしていません。監視用の画面からサービスを止められるべきではないためです。', '要解鎖：結束 LaunchAgent，從 [[~/.config/deployboard/config.json]] 或代理的 [[ProgramArguments]] 移除 [[--read-only]]，然後再啟動。故意不做成畫面開關 — 一個監察用的畫面不應該被人說服去結束服務。', '要解锁：退出 LaunchAgent，从 [[~/.config/deployboard/config.json]] 或代理的 [[ProgramArguments]] 中去掉 [[--read-only]]，然后重新启动。故意不做成界面开关 — 一个监控界面不应当能被说服去结束服务。'],
  ['settings.writeAllowed', 'Write mode — actions allowed', '書き込みモード — 操作できます', '寫入模式 — 允許操作', '写入模式 — 允许操作'],

  ['source.auto', 'auto', '自動', '自動', '自动'],
  ['source.autoPath', 'auto (path)', '自動（パス）', '自動（路徑）', '自动（路径）'],
  ['source.listed', 'listed', 'リスト', '已列出', '已列出'],
  ['source.unclassified', 'unclassified', '未分類', '未分類', '未分类'],

  ['status.all', 'All', 'すべて', '全部', '全部'],
  ['status.completed', 'Completed', '完了', '已完成', '已完成'],
  ['status.disabled', 'Disabled', '無効', '已停用', '已禁用'],
  ['status.error', 'Error', 'エラー', '錯誤', '错误'],
  ['status.offline', 'Offline', 'オフライン', '離線', '离线'],
  ['status.running', 'Running', '実行中', '運行中', '运行中'],
  ['status.scheduled', 'Scheduled', '予定', '已排程', '已计划'],
  ['status.stopped', 'Stopped', '停止', '已停止', '已停止'],

  ['storm.detail', '{summary} — usually a crash loop, a missing dependency, or a port already in use. Logs first.', '{summary} — 多くはクラッシュのループ、依存関係の不足、または使用中のポートです。まずログを見てください。', '{summary} — 通常是崩潰循環、缺少依賴，或者埠已被佔用。先看日誌。', '{summary} — 通常是崩溃循环、缺少依赖，或端口已被占用。先看日志。'],
  ['drift.one', '1 job drifted from desired state', '希望状態からずれたジョブが 1 件', '有 1 個工作偏離期望狀態', '有 1 个任务偏离期望状态'],
  ['drift.many', '{n} jobs drifted from desired state', '希望状態からずれたジョブが {n} 件', '有 {n} 個工作偏離期望狀態', '有 {n} 个任务偏离期望状态'],
  ['drift.detail', '{label}: want {expected}, have {actual}', '{label}: 期待 {expected}、実際 {actual}', '{label}：期望 {expected}，實際 {actual}', '{label}：期望 {expected}，实际 {actual}'],
  ['drift.align', 'Align', '揃える', '對齊', '对齐'],
  ['drift.alignTitle', 'Align this job to desired state?', 'このジョブを希望状態に揃えますか？', '將此工作對齊到期望狀態？', '将此任务对齐到期望状态？'],
  ['drift.alignNote', 'Will run {action} so the job becomes {expected}.', '{action} を実行して {expected} にします。', '將執行 {action}，使工作成為 {expected}。', '将执行 {action}，使任务成为 {expected}。'],
  ['drift.toast', '{action} on {label}', '{label} に {action}', '對 {label} 執行 {action}', '对 {label} 执行 {action}'],
  ['drift.toastFail', 'Align {label} failed: {message}', '{label} の揃えに失敗: {message}', '對齊 {label} 失敗：{message}', '对齐 {label} 失败：{message}'],
  ['drift.badge', '{n} drift', 'ずれ {n}', '偏離 {n}', '偏离 {n}'],
  ['drift.badgeTitle', 'Ours jobs whose live state does not match desired config', '希望設定と一致しない Ours ジョブ', '實際狀態與期望設定不符的 Ours 工作', '实际状态与期望配置不符的 Ours 任务'],
  ['storm.group', 'Group: {group}', 'グループ: {group}', '群組：{group}', '分组：{group}'],
  ['storm.job', '{n} job', '{n} 件のジョブ', '{n} 個工作', '{n} 个任务'],
  ['storm.jobs', '{n} jobs', '{n} 件のジョブ', '{n} 個工作', '{n} 个任务'],
  ['storm.loop', 'in a restart loop', 'が再起動ループ中', '陷入重啟循環', '处于重启循环'],
  ['storm.more', '+{n} more', 'ほか {n} 件', '另外 {n} 個', '另外 {n} 个'],
  ['storm.repeating', 'restarting repeatedly', 'が繰り返し再起動しています', '正在反覆重啟', '正在反复重启'],
  ['storm.retireNote', 'launchctl disable for every job in that group — they will not start again, including at login. The plists stay on disk and Enable undoes it.', 'そのグループのすべてのジョブに launchctl disable を実行します。ログイン時も含め、再起動しません。plist は残り、有効化で元に戻せます。', '對該群組的每個工作執行 launchctl disable — 包括登入時都不會再啟動。plist 仍留在磁碟，啟用就可以還原。', '对该分组的每个任务执行 launchctl disable — 包括登录时也不会再启动。plist 仍留在磁盘上，启用即可还原。'],
  ['storm.retireThem', 'Retire them', '無効化する', '停用它們', '退役它们'],
  ['storm.retireTitle', 'Retire the churning jobs?', '再起動を繰り返しているジョブを無効化しますか？', '停用這些反覆重啟的工作？', '退役这些反复重启的任务？'],
  ['storm.stopNote', 'Stops every running job in that group. KeepAlive jobs are unloaded so they stay down until you start them again.', 'そのグループで実行中のジョブをすべて停止します。KeepAlive のジョブはアンロードするので、開始するまで止まったままです。', '停止該群組內所有運行中的工作。KeepAlive 的工作會被卸載，再次啟動之前會保持停止。', '停止该分组中所有正在运行的任务。KeepAlive 任务会被卸载，再次启动之前会保持停止。'],
  ['storm.stopThem', 'Stop them', '停止する', '停止它們', '停止它们'],
  ['storm.stopTitle', 'Stop the churning jobs?', '再起動を繰り返しているジョブを停止しますか？', '停止這些反覆重啟的工作？', '停止这些反复重启的任务？'],
  ['storm.toast', '{action} on {group}: {ok} done, {failed} failed', '{group} に {action}: {ok} 件完了、{failed} 件失敗', '對 {group} 執行 {action}：完成 {ok} 個，失敗 {failed} 個', '对 {group} 执行 {action}：完成 {ok} 个，失败 {failed} 个'],
  ['storm.toastFail', '{action} on {group} failed: {message}', '{group} の {action} に失敗しました: {message}', '對 {group} 執行 {action} 失敗：{message}', '对 {group} 执行 {action} 失败：{message}'],

  ['table.actions', 'Actions', '操作', '操作', '操作'],
  ['table.empty', 'No jobs found.', 'ジョブが見つかりません。', '找不到工作。', '未找到任务。'],
  ['table.exit', 'Exit', '終了', '退出', '退出'],
  ['table.jobCount', '{n} jobs', '{n} 件', '{n} 個工作', '{n} 个任务'],
  ['table.label', 'Label', 'ラベル', '標籤', '标签'],
  ['table.pid', 'PID', 'PID', 'PID', 'PID'],
  ['table.runs', 'Runs', '回数', '次數', '次数'],
  ['table.ungrouped', 'Ungrouped', '未分類', '未分組', '未分组'],

  ['theme.dark', 'Dark', 'ダーク', '深色', '深色'],
  ['theme.label', 'Theme', 'テーマ', '主題', '主题'],
  ['theme.light', 'Light', 'ライト', '淺色', '浅色'],
  ['theme.system', 'System', 'システム', '系統', '系统'],

  ['toast.alertToggleFailed', 'Alert toggle failed: {message}', '通知の切り替えに失敗しました: {message}', '切換通知失敗：{message}', '切换通知失败：{message}'],
  ['toast.alertsDisabled', 'Alerts disabled for {label}', '{label} の通知を無効にしました', '已關閉 {label} 的通知', '已关闭 {label} 的通知'],
  ['toast.alertsEnabled', 'Alerts enabled for {label}', '{label} の通知を有効にしました', '已開啟 {label} 的通知', '已开启 {label} 的通知'],
  ['toast.backAuto', '{label} back to auto', '{label} を自動に戻しました', '已將 {label} 改回自動', '已将 {label} 改回自动'],
  ['toast.classifyFailed', 'Classify failed: {message}', '分類に失敗しました: {message}', '分類失敗：{message}', '分类失败：{message}'],
  ['toast.close', 'Close', '閉じる', '關閉', '关闭'],
  ['toast.couldNotVerify', 'could not verify {action}: {message}', '{action} を確認できませんでした: {message}', '無法確認 {action}：{message}', '无法确认 {action}：{message}'],
  ['toast.failed', '{action} failed: {message}', '{action} に失敗しました: {message}', '{action} 失敗：{message}', '{action} 失败：{message}'],
  ['toast.notTake', '{action} did not take: {verdict} (now {state})', '{action} は反映されませんでした: {verdict}（現在 {state}）', '{action} 沒有生效：{verdict}（目前 {state}）', '{action} 没有生效：{verdict}（当前 {state}）'],
  ['toast.notVerified', 'not verified: {action} did not stick — still {state}', '未確認: {action} は定着しませんでした — まだ {state}', '未確認：{action} 沒有保持 — 仍然是 {state}', '未确认：{action} 没有保持 — 仍是 {state}'],
  ['toast.sent', '{action} sent: {label}', '{action} を送信しました: {label}', '已送出 {action}：{label}', '已发送 {action}：{label}'],
  ['toast.sentNote', '{action} sent: {label} — {note}', '{action} を送信しました: {label} — {note}', '已送出 {action}：{label} — {note}', '已发送 {action}：{label} — {note}'],
  ['toast.testFailed', 'Test alert failed: {message}', 'テスト通知に失敗しました: {message}', '測試通知失敗：{message}', '测试通知失败：{message}'],
  ['toast.testSent', 'Test alert sent for {label}', '{label} のテスト通知を送信しました', '已傳送 {label} 的測試通知', '已发送 {label} 的测试通知'],
  ['toast.unexpected', 'unexpected state', '予期しない状態', '未預期的狀態', '意外状态'],
  ['toast.verified', 'verified: {action} — {state} ({seconds}s later)', '確認しました: {action} — {state}（{seconds}秒後）', '已確認：{action} — {state}（{seconds} 秒後）', '已确认：{action} — {state}（{seconds} 秒后）'],

  ['tooltip.lastNoLog', 'Last run: unknown (no log path configured)', '前回の実行: 不明（ログのパスが未設定）', '上次執行：未知（沒有設定日誌路徑）', '上次运行：未知（未配置日志路径）'],
  ['tooltip.lastRun', 'Last run: {when}', '前回の実行: {when}', '上次執行：{when}', '上次运行：{when}'],
  ['tooltip.lastUnknown', 'Last run: unknown', '前回の実行: 不明', '上次執行：未知', '上次运行：未知'],
  ['tooltip.nextRun', 'Next run: {when}', '次回の実行: {when}', '下次執行：{when}', '下次运行：{when}'],

  ['verb.cancel', 'Cancel', 'キャンセル', '取消', '取消'],
  ['verb.close', 'Close', '閉じる', '關閉', '关闭'],
  ['verb.diagnose', 'Diagnose', '診断', '診斷', '诊断'],
  ['verb.disable', 'Disable', '無効化', '停用', '禁用'],
  ['verb.enable', 'Enable', '有効化', '啟用', '启用'],
  ['verb.logs', 'Logs', 'ログ', '日誌', '日志'],
  ['verb.reload', 'Reload', '再読み込み', '重新載入', '重新加载'],
  ['verb.restart', 'Restart', '再起動', '重啟', '重启'],
  ['verb.retire', 'Retire', '無効化', '停用', '退役'],
  ['verb.save', 'Save', '保存', '儲存', '保存'],
  ['verb.saving', 'Saving…', '保存しています…', '儲存中…', '正在保存…'],
  ['verb.start', 'Start', '開始', '啟動', '启动'],
  ['verb.stop', 'Stop', '停止', '停止', '停止'],
];

const seenKeys = new Set();
for (const row of ROWS) {
  if (!Array.isArray(row) || row.length !== 5 || row.some((cell) => typeof cell !== 'string' || cell === '')) {
    throw new Error('i18n row must have 5 non-empty strings: ' + (row && row[0]));
  }
  if (seenKeys.has(row[0])) throw new Error('duplicate i18n key: ' + row[0]);
  seenKeys.add(row[0]);
}

function buildCatalog(index) {
  const out = {};
  for (const row of ROWS) out[row[0]] = row[index];
  return out;
}

export const en = buildCatalog(1);
export const ja = buildCatalog(2);
export const zhHant = buildCatalog(3);
export const zhHans = buildCatalog(4);

export const catalogs = {
  en,
  ja,
  'zh-Hant': zhHant,
  'zh-Hans': zhHans,
};

/** @type {import('@preact/signals').Signal<string>} */
export const locale = signal('en');

/** Current locale code. Reading this subscribes the caller to changes. */
export function getLocale() {
  return locale.value || 'en';
}

function isSupported(code) {
  return LOCALES.includes(code);
}

function readStored() {
  try {
    if (typeof localStorage === 'undefined') return null;
    const current = localStorage.getItem(LOCALE_KEY);
    if (current) return current;
    // One release: adopt the previous namespace when the new key is absent.
    const legacy = localStorage.getItem('launch-pilot:locale');
    if (legacy) {
      try { localStorage.setItem(LOCALE_KEY, legacy); } catch { /* private browsing */ }
      return legacy;
    }
    return null;
  } catch {
    return null;
  }
}

function readLanguage() {
  try {
    if (typeof navigator === 'undefined' || !navigator) return '';
    return navigator.language || '';
  } catch {
    return '';
  }
}

/**
 * Map a BCP 47 tag: ja* → ja; zh-TW / zh-HK / zh-Hant / zh-MO → zh-Hant;
 * zh-CN / zh-SG / zh-Hans / any other zh* → zh-Hans; otherwise en.
 */
export function mapLanguage(language) {
  const lower = String(language || '').trim().toLowerCase().replace(/_/g, '-');
  if (!lower) return 'en';
  if (lower.startsWith('ja')) return 'ja';
  if (lower.startsWith('zh')) {
    if (
      lower.startsWith('zh-tw') ||
      lower.startsWith('zh-hk') ||
      lower.startsWith('zh-mo') ||
      lower.includes('hant')
    ) return 'zh-Hant';
    return 'zh-Hans';
  }
  return 'en';
}

/**
 * localStorage['deployboard:locale'] → navigator.language → en.
 * Pass { stored, language } to test without touching the environment.
 * A key omitted from the object is read from the environment; null/'' stored
 * values fall through to the language tag.
 */
export function resolveInitialLocale(env) {
  try {
    const hasStored = env && typeof env === 'object' && Object.prototype.hasOwnProperty.call(env, 'stored');
    const hasLanguage = env && typeof env === 'object' && Object.prototype.hasOwnProperty.call(env, 'language');
    const stored = hasStored ? env.stored : readStored();
    if (isSupported(stored)) return stored;
    const language = hasLanguage ? env.language : readLanguage();
    return mapLanguage(language);
  } catch {
    return 'en';
  }
}

function applyDocumentLang(code) {
  try {
    if (typeof document !== 'undefined' && document.documentElement) {
      document.documentElement.lang = code;
    }
  } catch { /* node tests, or a document that is not writable */ }
}

/**
 * Switch locale. Unknown codes are ignored (the current locale stays).
 * persist=false applies the choice without writing localStorage — used on
 * boot so an auto-detected language does not stick until the user picks one.
 */
export function setLocale(code, persist = true) {
  try {
    if (!isSupported(code)) return getLocale();
    locale.value = code;
    if (persist) {
      try { localStorage.setItem(LOCALE_KEY, code); } catch { /* private browsing */ }
    }
    applyDocumentLang(code);
    return code;
  } catch {
    return 'en';
  }
}

function lookup(code, key) {
  const cat = catalogs[code];
  if (!cat) return undefined;
  const value = cat[key];
  if (typeof value === 'string' && value !== '') return value;
  return undefined;
}

function interpolate(str, vars) {
  if (!vars || typeof vars !== 'object') return str;
  return str.replace(/\{(\w+)\}/g, (match, name) => {
    if (!Object.prototype.hasOwnProperty.call(vars, name) || vars[name] == null) return '';
    return String(vars[name]);
  });
}

/**
 * Translate `key` for the active locale. Falls back to English, then to the
 * key. `vars` replaces `{name}` placeholders. Never blank, never throws.
 */
export function t(key, vars) {
  try {
    const k = typeof key === 'string' ? key : (key == null ? '' : String(key));
    const code = getLocale();
    let str = lookup(code, k);
    if (str == null) str = lookup('en', k);
    if (str == null) str = k || '\u2026';
    str = interpolate(str, vars);
    if (typeof str !== 'string' || str === '') return k || '\u2026';
    return str;
  } catch {
    return typeof key === 'string' && key ? key : '\u2026';
  }
}

/**
 * Split a translated string on [[code]] markers. Odd segments are the code
 * token; even segments (and bare strings) are prose. Calls t(), so a render
 * that walks the result stays subscribed to the locale signal.
 */
export function codeSegments(key, vars) {
  try {
    const str = t(key, vars);
    const parts = [];
    const re = /\[\[([^\]]+)\]\]/g;
    let last = 0;
    let m;
    while ((m = re.exec(str)) !== null) {
      if (m.index > last) parts.push(str.slice(last, m.index));
      parts.push({ code: m[1] });
      last = re.lastIndex;
    }
    if (last < str.length) parts.push(str.slice(last));
    if (parts.length === 0) parts.push(str || key || '\u2026');
    return parts;
  } catch {
    return [typeof key === 'string' && key ? key : '\u2026'];
  }
}

const DIAG_EXACT_MSG = {
  'No Program or ProgramArguments configured': 'diag.msg.noProgram',
  'No program path to check': 'diag.msg.noProgramPath',
  'No plist path known': 'diag.msg.noPlist',
  'Cannot determine file owner on this platform': 'diag.msg.noOwnerPlatform',
  'No log paths configured': 'diag.msg.noLogs',
  'Log parent directories exist': 'diag.msg.logDirsExist',
};

const DIAG_EXACT_SUG = {
  'Add Program or ProgramArguments to the plist': 'diag.sug.addProgram',
  'Verify the path in Program or ProgramArguments': 'diag.sug.verifyPath',
  'Check program logs for details': 'diag.sug.checkLogs',
  'Verify ProgramArguments in plist': 'diag.sug.usage',
  'Check plist encoding (UTF-8); ensure Full Disk Access for Terminal': 'diag.sug.io',
  'Check executable and working directory permissions': 'diag.sug.checkPerms',
  'Verify Program/ProgramArguments path exists': 'diag.sug.verifyProgram',
};

const DIAG_MEANINGS = {
  'Normal exit': 'diag.meaning.normal',
  'General error': 'diag.meaning.general',
  'Command usage error': 'diag.meaning.usage',
  'I/O error': 'diag.meaning.io',
  'Permission denied': 'diag.meaning.permission',
  'Configuration error (EX_CONFIG)': 'diag.meaning.config',
  'Command not executable': 'diag.meaning.notExec',
  'Command not found': 'diag.meaning.notFound',
};

function translateMeaning(meaning) {
  if (DIAG_MEANINGS[meaning]) return t(DIAG_MEANINGS[meaning]);
  let m;
  if ((m = /^Killed by signal (\d+) \(([^)]+)\)$/.exec(meaning))) {
    return t('diag.meaning.killedNamed', { n: m[1], name: m[2] });
  }
  if ((m = /^Killed by signal (\d+)$/.exec(meaning))) {
    return t('diag.meaning.killed', { n: m[1] });
  }
  if ((m = /^Unknown exit code (\d+)$/.exec(meaning))) {
    return t('diag.meaning.unknown', { n: m[1] });
  }
  return meaning;
}

function translateDiagMessage(message) {
  if (!message) return message;
  if (DIAG_EXACT_MSG[message]) return t(DIAG_EXACT_MSG[message]);
  let m;
  if ((m = /^Exit code (-?\d+): (.*)$/.exec(message))) {
    return t('diag.msg.exit', { code: m[1], meaning: translateMeaning(m[2]) });
  }
  if ((m = /^Cannot stat (.+): (.+)$/.exec(message))) {
    return t('diag.msg.cannotStat', { path: m[1], err: m[2] });
  }
  if ((m = /^(.+) lacks execute permission$/.exec(message))) {
    return t('diag.msg.noExec', { path: m[1] });
  }
  if ((m = /^(.+) owned by UID (\d+), not current user UID (\d+)$/.exec(message))) {
    return t('diag.msg.ownedByUid', { path: m[1], uid: m[2], current: m[3] });
  }
  if ((m = /^(.+) owned by current user$/.exec(message))) {
    return t('diag.msg.ownedByUser', { path: m[1] });
  }
  if ((m = /^(.+) has permissions ([0-7]+) \(group\/world writable\)$/.exec(message))) {
    return t('diag.msg.permsBad', { path: m[1], perm: m[2] });
  }
  if ((m = /^(.+) permissions ([0-7]+) OK$/.exec(message))) {
    return t('diag.msg.permsOk', { path: m[1], perm: m[2] });
  }
  if ((m = /^Log parent directory missing: (.+)$/.exec(message))) {
    return t('diag.msg.logMissing', { dirs: m[1] });
  }
  if ((m = /^(.+) does not exist$/.exec(message))) {
    return t('diag.msg.notExists', { path: m[1] });
  }
  if ((m = /^(.+) exists$/.exec(message))) {
    return t('diag.msg.pathExists', { path: m[1] });
  }
  return message;
}

function translateDiagSuggestion(suggestion) {
  if (!suggestion) return suggestion;
  if (DIAG_EXACT_SUG[suggestion]) return t(DIAG_EXACT_SUG[suggestion]);
  let m;
  if ((m = /^Add execute permission: chmod \+x (.+)$/.exec(suggestion))) {
    return t('diag.sug.addExec', { path: m[1] });
  }
  if ((m = /^chmod \+x (.+)$/.exec(suggestion))) return t('diag.sug.chmod', { path: m[1] });
  if ((m = /^chown (\d+) (.+)$/.exec(suggestion))) return t('diag.sug.chown', { uid: m[1], path: m[2] });
  if ((m = /^chmod 644 (.+)$/.exec(suggestion))) return t('diag.sug.chmod644', { path: m[1] });
  if ((m = /^mkdir -p (.+)$/.exec(suggestion))) return t('diag.sug.mkdir', { path: m[1] });
  if ((m = /^Validate plist: plutil -lint (.+)$/.exec(suggestion))) return t('diag.sug.plutil', { path: m[1] });
  return suggestion;
}

/**
 * Translate a diagnostic check from the API. English is left exactly as the
 * server sent it (paths, commands, exit text). Other locales translate the
 * six check names and the known message/suggestion templates; anything
 * unmatched stays as the server string so a description is never blanked out.
 */
export function localizeCheck(check) {
  try {
    if (!check || typeof check !== 'object') return check;
    if (getLocale() === 'en') return check;
    const nameKey = check.id ? `diag.name.${check.id}` : '';
    const translatedName = nameKey && lookup('en', nameKey) ? t(nameKey) : '';
    const message = translateDiagMessage(check.message);
    const suggestion = check.suggestion ? translateDiagSuggestion(check.suggestion) : check.suggestion;
    return {
      ...check,
      name: translatedName || check.name || check.id || '\u2026',
      message: message || check.message || '',
      suggestion: suggestion || check.suggestion,
    };
  } catch {
    return check;
  }
}
