import { DashboardWelcome } from './dashboard/DashboardWelcome'
import { DashboardStorage } from './dashboard/DashboardStorage'
import { DashboardRecentFolders } from './dashboard/DashboardRecentFolders'
import { DashboardRecentFiles } from './dashboard/DashboardRecentFiles'
import { DashboardRecentActivity } from './dashboard/DashboardRecentActivity'

// DashboardPage is the post-login landing: a workspace overview that
// composes the welcome header, storage cards, and recent activity sections.
export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <DashboardWelcome />
      <DashboardStorage />
      <div className="grid gap-6 lg:grid-cols-2">
        <DashboardRecentFolders />
        <DashboardRecentFiles />
      </div>
      <DashboardRecentActivity />
    </div>
  )
}
