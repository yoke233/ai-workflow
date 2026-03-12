import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Settings2, Bot, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { SandboxSupportPanel } from "@/components/SandboxSupportPanel";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  Dialog, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter,
} from "@/components/ui/dialog";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { cn } from "@/lib/utils";
import { getErrorMessage } from "@/lib/v2Workbench";
import type { AgentDriver, AgentProfile } from "@/types/apiV2";
import type { SandboxSupportResponse } from "@/types/system";

const roleBadgeVariant: Record<string, "info" | "warning" | "default" | "secondary"> = {
  worker: "info",
  gate: "warning",
  lead: "default",
  support: "secondary",
};

const ALL_CAPS = ["fs_read", "fs_write", "terminal"] as const;

export function AgentsPage() {
  const { t } = useTranslation();
  const { apiClient } = useWorkbench();
  const [drivers, setDrivers] = useState<AgentDriver[]>([]);
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [sandboxSupport, setSandboxSupport] = useState<SandboxSupportResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [sandboxLoading, setSandboxLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sandboxError, setSandboxError] = useState<string | null>(null);

  const [driverDialogOpen, setDriverDialogOpen] = useState(false);
  const [profileDialogOpen, setProfileDialogOpen] = useState(false);

  const [driverName, setDriverName] = useState("");
  const [driverCmd, setDriverCmd] = useState("");
  const [driverArgs, setDriverArgs] = useState("");
  const [driverCaps, setDriverCaps] = useState<string[]>(["fs_read", "fs_write", "terminal"]);

  const [profileName, setProfileName] = useState("");
  const [profileRole, setProfileRole] = useState("worker");
  const [profileDriver, setProfileDriver] = useState("");
  const [profileCaps, setProfileCaps] = useState("backend,frontend");
  const [profileActions, setProfileActions] = useState("read_context,search_files,fs_write,terminal,submit,mark_blocked,request_help");
  const [profileMaxTurns, setProfileMaxTurns] = useState("12");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [driverResp, profileResp] = await Promise.all([
        apiClient.listDrivers(),
        apiClient.listProfiles(),
      ]);
      setDrivers(driverResp);
      setProfiles(profileResp);
      setProfileDriver((current) => current || driverResp[0]?.id || "");
    } catch (loadError) {
      setError(getErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  };

  const loadSandboxSupport = async () => {
    setSandboxLoading(true);
    setSandboxError(null);
    try {
      setSandboxSupport(await apiClient.getSandboxSupport());
    } catch (loadError) {
      setSandboxError(getErrorMessage(loadError));
    } finally {
      setSandboxLoading(false);
    }
  };

  useEffect(() => {
    void Promise.all([load(), loadSandboxSupport()]);
  }, []);

  const toggleCap = (cap: string) => {
    setDriverCaps((prev) =>
      prev.includes(cap) ? prev.filter((item) => item !== cap) : [...prev, cap],
    );
  };

  const createDriver = async () => {
    try {
      await apiClient.createDriver({
        id: driverName.trim(),
        launch_command: driverCmd.trim(),
        launch_args: driverArgs.split(" ").map((item) => item.trim()).filter(Boolean),
        capabilities_max: {
          fs_read: driverCaps.includes("fs_read"),
          fs_write: driverCaps.includes("fs_write"),
          terminal: driverCaps.includes("terminal"),
        },
      });
      setDriverDialogOpen(false);
      setDriverName("");
      setDriverCmd("");
      setDriverArgs("");
      await load();
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    }
  };

  const createProfile = async () => {
    try {
      await apiClient.createProfile({
        id: profileName.trim(),
        name: profileName.trim(),
        driver_id: profileDriver,
        role: profileRole,
        capabilities: profileCaps.split(",").map((item) => item.trim()).filter(Boolean),
        actions_allowed: profileActions.split(",").map((item) => item.trim()).filter(Boolean),
        session: {
          reuse: true,
          max_turns: Number.parseInt(profileMaxTurns, 10) || 12,
        },
      });
      setProfileDialogOpen(false);
      setProfileName("");
      await load();
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    }
  };

  return (
    <div className="flex-1 space-y-6 p-8">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">{t("agents.title")}</h1>
            {loading ? <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /> : null}
          </div>
          <p className="text-sm text-muted-foreground">{t("agents.subtitle")}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setDriverDialogOpen(true)}>
            <Settings2 className="mr-2 h-4 w-4" />
            {t("agents.newDriver")}
          </Button>
          <Button onClick={() => setProfileDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t("agents.newProfile")}
          </Button>
        </div>
      </div>

      {error ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

      <SandboxSupportPanel
        report={sandboxSupport}
        loading={sandboxLoading}
        error={sandboxError}
        onRefresh={() => {
          void loadSandboxSupport();
        }}
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Bot className="h-5 w-5" />
            {t("agents.drivers")} <Badge variant="secondary" className="ml-1">{drivers.length}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("agents.driverName")}</TableHead>
                <TableHead>{t("agents.launchCommand")}</TableHead>
                <TableHead>{t("agents.maxCapabilities")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {drivers.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center text-muted-foreground">{t("agents.noDrivers")}</TableCell>
                </TableRow>
              ) : (
                drivers.map((driver) => (
                  <TableRow key={driver.id}>
                    <TableCell className="font-medium">{driver.id}</TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                        {driver.launch_command} {(driver.launch_args ?? []).join(" ")}
                      </code>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {ALL_CAPS.filter((cap) => driver.capabilities_max[cap]).map((cap) => (
                          <Badge key={cap} variant="outline" className="text-xs">{cap}</Badge>
                        ))}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            {t("agents.profiles")} <Badge variant="secondary" className="ml-1">{profiles.length}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("agents.driverName")}</TableHead>
                <TableHead>{t("agents.role")}</TableHead>
                <TableHead>{t("agents.boundDriver")}</TableHead>
                <TableHead>{t("agents.capabilityTags")}</TableHead>
                <TableHead>{t("agents.actionPermissions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {profiles.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">{t("agents.noProfiles")}</TableCell>
                </TableRow>
              ) : (
                profiles.map((profile) => (
                  <TableRow key={profile.id}>
                    <TableCell className="font-medium">{profile.name || profile.id}</TableCell>
                    <TableCell>
                      <Badge variant={roleBadgeVariant[profile.role] ?? "secondary"}>{profile.role}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{profile.driver_id}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(profile.capabilities ?? []).map((capability) => (
                          <Badge key={capability} variant="outline" className="text-xs">{capability}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(profile.actions_allowed ?? []).map((action) => (
                          <Badge key={action} variant="secondary" className="text-xs">{action}</Badge>
                        ))}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={driverDialogOpen} onClose={() => setDriverDialogOpen(false)} className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("agents.newDriver")}</DialogTitle>
          <DialogDescription>{t("agents.createDriverDesc")}</DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.driverId")}</label>
            <Input value={driverName} onChange={(event) => setDriverName(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.launchCommand")}</label>
            <Input value={driverCmd} onChange={(event) => setDriverCmd(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.launchArgs")}</label>
            <Input value={driverArgs} onChange={(event) => setDriverArgs(event.target.value)} />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("agents.maxCapabilities")}</label>
            <div className="flex gap-4">
              {ALL_CAPS.map((cap) => (
                <label key={cap} className="flex cursor-pointer items-center gap-2">
                  <button
                    type="button"
                    onClick={() => toggleCap(cap)}
                    className={cn(
                      "flex h-[18px] w-[18px] items-center justify-center rounded transition-colors",
                      driverCaps.includes(cap)
                        ? "bg-primary text-primary-foreground"
                        : "border border-input",
                    )}
                  >
                    {driverCaps.includes(cap) ? "✓" : ""}
                  </button>
                  <span className="text-sm">{cap}</span>
                </label>
              ))}
            </div>
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => setDriverDialogOpen(false)}>{t("common.cancel")}</Button>
          <Button onClick={() => void createDriver()}>{t("agents.createDriver")}</Button>
        </DialogFooter>
      </Dialog>

      <Dialog open={profileDialogOpen} onClose={() => setProfileDialogOpen(false)} className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("agents.newProfile")}</DialogTitle>
          <DialogDescription>{t("agents.createProfileDesc")}</DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("agents.profileId")}</label>
              <Input value={profileName} onChange={(event) => setProfileName(event.target.value)} />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("agents.role")}</label>
              <select
                className="flex h-10 w-full rounded-md border bg-background px-3 text-sm"
                value={profileRole}
                onChange={(event) => setProfileRole(event.target.value)}
              >
                <option value="lead">lead</option>
                <option value="worker">worker</option>
                <option value="gate">gate</option>
                <option value="support">support</option>
              </select>
            </div>
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.bindDriver")}</label>
            <select
              className="flex h-10 w-full rounded-md border bg-background px-3 text-sm"
              value={profileDriver}
              onChange={(event) => setProfileDriver(event.target.value)}
            >
              {drivers.map((driver) => (
                <option key={driver.id} value={driver.id}>{driver.id}</option>
              ))}
            </select>
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.capabilityTagsComma")}</label>
            <Input value={profileCaps} onChange={(event) => setProfileCaps(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.allowedActionsComma")}</label>
            <Input value={profileActions} onChange={(event) => setProfileActions(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.maxTurns")}</label>
            <Input value={profileMaxTurns} onChange={(event) => setProfileMaxTurns(event.target.value)} />
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => setProfileDialogOpen(false)}>{t("common.cancel")}</Button>
          <Button onClick={() => void createProfile()}>{t("agents.createProfile")}</Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}


