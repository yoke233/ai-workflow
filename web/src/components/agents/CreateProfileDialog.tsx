import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";
import { Select, SelectItem } from "@/components/ui/select";
import type { AgentDriver, AgentProfile } from "@/types/apiV2";
import type { LLMConfigItem } from "@/types/system";

interface Props {
  open: boolean;
  drivers: AgentDriver[];
  llmConfigs: LLMConfigItem[];
  onClose: () => void;
  onCreate: (payload: AgentProfile) => Promise<void>;
}

export function CreateProfileDialog({ open, drivers, llmConfigs, onClose, onCreate }: Props) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [role, setRole] = useState("worker");
  const [driverId, setDriverId] = useState(() => drivers[0]?.id ?? "");
  const [llmConfigID, setLLMConfigID] = useState(() => llmConfigs[0]?.id ?? "");
  const [caps, setCaps] = useState("backend,frontend");
  const [actions, setActions] = useState("read_context,search_files,fs_write,terminal,submit,mark_blocked,request_help");
  const [maxTurns, setMaxTurns] = useState("12");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    if (!driverId && drivers[0]?.id) {
      setDriverId(drivers[0].id);
    }
    if (!llmConfigID && llmConfigs[0]?.id) {
      setLLMConfigID(llmConfigs[0].id);
    }
  }, [open, driverId, drivers, llmConfigID, llmConfigs]);

  const handleClose = () => {
    setName("");
    setRole("worker");
    setDriverId(drivers[0]?.id ?? "");
    setLLMConfigID(llmConfigs[0]?.id ?? "");
    setCaps("backend,frontend");
    setActions("read_context,search_files,fs_write,terminal,submit,mark_blocked,request_help");
    setMaxTurns("12");
    onClose();
  };

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      const selectedDriver = drivers.find((driver) => driver.id === driverId);
      if (!selectedDriver) {
        return;
      }
      await onCreate({
        id: name.trim(),
        name: name.trim(),
        driver_id: driverId,
        llm_config_id: llmConfigID || undefined,
        driver: {
          launch_command: selectedDriver.launch_command,
          launch_args: selectedDriver.launch_args,
          env: selectedDriver.env,
          capabilities_max: selectedDriver.capabilities_max,
        },
        role,
        capabilities: caps.split(",").map((s) => s.trim()).filter(Boolean),
        actions_allowed: actions.split(",").map((s) => s.trim()).filter(Boolean),
        session: { reuse: true, max_turns: Number.parseInt(maxTurns, 10) || 12 },
      });
      handleClose();
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={handleClose} className="max-w-lg">
      <DialogHeader>
        <DialogTitle>{t("agents.newProfile")}</DialogTitle>
        <DialogDescription>{t("agents.createProfileDesc")}</DialogDescription>
      </DialogHeader>
      <DialogBody>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.profileId")}</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("agents.role")}</label>
            <Select value={role} onValueChange={setRole}>
              <SelectItem value="lead">lead</SelectItem>
              <SelectItem value="worker">worker</SelectItem>
              <SelectItem value="gate">gate</SelectItem>
              <SelectItem value="support">support</SelectItem>
            </Select>
          </div>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">{t("agents.bindDriver")}</label>
          <Select value={driverId} onValueChange={setDriverId}>
            {drivers.map((d) => (
              <SelectItem key={d.id} value={d.id}>{d.id}</SelectItem>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">
            {t("agents.bindLLMConfig")}
            <span className="ml-1 text-xs font-normal text-muted-foreground">({t("agents.optionalField")})</span>
          </label>
          <Select value={llmConfigID} onValueChange={setLLMConfigID}>
            <SelectItem value="">-</SelectItem>
            {llmConfigs.map((item) => (
              <SelectItem key={item.id} value={item.id}>{item.id}</SelectItem>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">{t("agents.capabilityTagsComma")}</label>
          <Input value={caps} onChange={(e) => setCaps(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">{t("agents.allowedActionsComma")}</label>
          <Input value={actions} onChange={(e) => setActions(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">{t("agents.maxTurns")}</label>
          <Input value={maxTurns} onChange={(e) => setMaxTurns(e.target.value)} />
        </div>
      </DialogBody>
      <DialogFooter>
        <Button variant="outline" onClick={handleClose}>{t("common.cancel")}</Button>
        <Button onClick={() => void handleCreate()} disabled={!name.trim() || !driverId || submitting}>
          {t("agents.createProfile")}
        </Button>
      </DialogFooter>
    </Dialog>
  );
}
