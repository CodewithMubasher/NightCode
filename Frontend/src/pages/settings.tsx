import { useState } from "react"
import {
  UserIcon,
  ShieldIcon,
  PaletteIcon,
  BellIcon,
  InfoIcon,
  KeyIcon,
} from "lucide-react"

const navItems = [
  { icon: UserIcon, label: "Profile", id: "profile" },
  { icon: ShieldIcon, label: "Account", id: "account" },
  { icon: KeyIcon, label: "API Keys", id: "api-keys" },
  { icon: PaletteIcon, label: "Appearance", id: "appearance" },
  { icon: BellIcon, label: "Notifications", id: "notifications" },
  { icon: InfoIcon, label: "About", id: "about" },
]

function ProfileTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">Profile</h2>
        <p className="text-sm text-white/40">This is how others will see you on the site.</p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm text-white/60">Name</label>
          <input
            type="text"
            defaultValue="Mubasher"
            className="w-full px-3 py-2 rounded-xl border border-white/10 bg-white/5 text-sm text-white/80 outline-none focus:border-white/20 transition-colors"
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm text-white/60">Email</label>
          <input
            type="email"
            defaultValue="mubasher@example.com"
            className="w-full px-3 py-2 rounded-xl border border-white/10 bg-white/5 text-sm text-white/80 outline-none focus:border-white/20 transition-colors"
          />
        </div>
      </div>

      <div className="space-y-2">
        <label className="text-sm text-white/60">Bio</label>
        <textarea
          placeholder="Tell us about yourself..."
          className="w-full px-3 py-2 rounded-xl border border-white/10 bg-white/5 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors min-h-[80px] resize-none"
        />
      </div>

      <button className="px-4 py-1.5 rounded-xl bg-[#a3004c] text-white text-sm font-medium hover:bg-[#a3004c]/80 transition-colors cursor-pointer">
        Save changes
      </button>
    </div>
  )
}

function AccountTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">Account</h2>
        <p className="text-sm text-white/40">Manage your account settings.</p>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white/80">Delete account</p>
            <p className="text-xs text-white/40">Permanently delete your account and all data</p>
          </div>
          <button className="px-3 py-1 rounded-xl border border-red-500/30 text-red-400 text-xs hover:bg-red-500/10 transition-colors cursor-pointer">
            Delete
          </button>
        </div>
      </div>
    </div>
  )
}

function ApiKeysTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">API Keys</h2>
        <p className="text-sm text-white/40">Manage your API keys for external services.</p>
      </div>

      <div className="space-y-4">
        <div className="space-y-2">
          <label className="text-sm text-white/60">OpenAI API Key</label>
          <input
            type="password"
            placeholder="sk-..."
            className="w-full px-3 py-2 rounded-xl border border-white/10 bg-white/5 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm text-white/60">GitHub Token</label>
          <input
            type="password"
            placeholder="ghp_..."
            className="w-full px-3 py-2 rounded-xl border border-white/10 bg-white/5 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
          />
        </div>
      </div>

      <button className="px-4 py-1.5 rounded-xl bg-[#a3004c] text-white text-sm font-medium hover:bg-[#a3004c]/80 transition-colors cursor-pointer">
        Save keys
      </button>
    </div>
  )
}

function AppearanceTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">Appearance</h2>
        <p className="text-sm text-white/40">Customize the look and feel.</p>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white/80">Theme</p>
            <p className="text-xs text-white/40">Select your preferred theme</p>
          </div>
          <span className="text-sm text-white/50">Dark</span>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white/80">Accent color</p>
            <p className="text-xs text-white/40">Primary brand color</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="size-5 rounded-full bg-[#a3004c] border-2 border-white/20" />
            <span className="text-sm text-white/50">#a3004c</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function NotificationsTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">Notifications</h2>
        <p className="text-sm text-white/40">Configure notification preferences.</p>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white/80">Email notifications</p>
            <p className="text-xs text-white/40">Receive email updates</p>
          </div>
          <div className="size-10 rounded-full border border-white/10 bg-white/5 flex items-center justify-center">
            <div className="size-4 rounded-full bg-[#a3004c]" />
          </div>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white/80">Push notifications</p>
            <p className="text-xs text-white/40">Receive push notifications</p>
          </div>
          <div className="size-10 rounded-full border border-white/10 bg-white/5 flex items-center justify-center">
            <div className="size-1.5 rounded-full bg-white/30" />
          </div>
        </div>
      </div>
    </div>
  )
}

function AboutTab() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-6 space-y-6">
      <div>
        <h2 className="text-lg font-medium text-white">About</h2>
        <p className="text-sm text-white/40">Information about NightCode.</p>
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-white/60">Version</span>
          <span className="text-sm text-white/80">1.0.0</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-sm text-white/60">Build</span>
          <span className="text-sm text-white/80">2026.09.12</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-sm text-white/60">Runtime</span>
          <span className="text-sm text-white/80">AI Agent Harness Demo</span>
        </div>
      </div>
    </div>
  )
}

const tabs: Record<string, () => JSX.Element> = {
  profile: ProfileTab,
  account: AccountTab,
  "api-keys": ApiKeysTab,
  appearance: AppearanceTab,
  notifications: NotificationsTab,
  about: AboutTab,
}

export function Settings() {
  const [activeTab, setActiveTab] = useState("profile")
  const ActiveTab = tabs[activeTab]

  return (
    <div className="flex flex-1 flex-col gap-6 px-14 py-10">
      <div>
        <h1 className="text-xl font-semibold text-white/90">Settings</h1>
        <p className="text-sm text-white/40 mt-1">
          Manage your account settings and set e-mail preferences.
        </p>
      </div>

      <div className="flex flex-col gap-6 md:flex-row">
        <nav className="flex flex-col gap-1 w-full md:w-[220px] shrink-0">
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => setActiveTab(item.id)}
              className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors cursor-pointer ${
                activeTab === item.id
                  ? "bg-white/10 text-white"
                  : "text-white/40 hover:text-white hover:bg-white/5"
              }`}
            >
              <item.icon className="size-4" />
              {item.label}
            </button>
          ))}
        </nav>

        <div className="flex-1 min-w-0">
          {ActiveTab && <ActiveTab />}
        </div>
      </div>
    </div>
  )
}
