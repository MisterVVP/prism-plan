import type { Settings, Command } from "../../types";

interface State {
  settings: Settings;
  commands: Command[];
}

export const initialState: State = {
  settings: { tasksPerCategory: 3, showDoneTasks: false },
  commands: [],
};

type SetSettingsAction = { type: "set-settings"; settings: Settings };
type MergeSettingsAction = { type: "merge-settings"; settings: Partial<Settings> };
type UpdateSettingsAction = {
  type: "update-settings";
  userId: string;
  settings: Partial<Settings>;
};
type ClearCommandsAction = { type: "clear-commands" };
type SetIdempotencyKeysAction = { type: "set-idempotency-keys"; keys: string[] };

type Action =
  | SetSettingsAction
  | MergeSettingsAction
  | UpdateSettingsAction
  | ClearCommandsAction
  | SetIdempotencyKeysAction;

export function settingsReducer(state: State = initialState, action: Action): State {
  switch (action.type) {
    case "set-settings":
      return { ...state, settings: action.settings };
    case "merge-settings":
      return { ...state, settings: { ...state.settings, ...action.settings } };
    case "update-settings": {
      const cmd: Command = {
        entityType: "user-settings",
        type: "update-user-settings",
        data: action.settings,
      };
      return {
        settings: { ...state.settings, ...action.settings },
        commands: [...state.commands, cmd],
      };
    }
    case "clear-commands":
      return { ...state, commands: [] };
    case "set-idempotency-keys": {
      let j = 0;
      const cmds = state.commands.map((c) => {
        if (c.idempotencyKey) return c;
        const key = action.keys[j++];
        return key ? { ...c, idempotencyKey: key } : c;
      });
      return { ...state, commands: cmds };
    }
    default:
      return state;
  }
}

export type { State, Action };
