export const suggestions = [
  {
    value: `SELECT * FROM shadow`,
    label: "SELECT All",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow ORDER BY createdAt`,
    label: "Sort By Created Time",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow ORDER BY updatedAt DESC`,
    label: "Sort By Updated DESC",
    autoTrigger: true,
  },
  // {
  //   value: `SELECT * FROM shadow ORDER BY updatedAt DESC LIMIT 1`,
  //   label: "Select the last Updated Thing",
  //   autoTrigger: true,
  // },
  {
    value: `SELECT thingId, connected, \`state.reported\` as reported, \`state.desired\` as desired, updatedAt FROM shadow`,
    label: "SELECT Status",
    autoTrigger: true,
  },
  {
    value: `SELECT thingId, connected, \`state.reported\`, createdAt as created_time, updatedAt as updated_time, \`tags\` FROM shadow`,
    label: "Rename Time Fields",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow WHERE connected = true`,
    label: "All Connected",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow
         WHERE connected = false
         OR connected IS NULL`,
    label: "Disconnected",
    autoTrigger: true,
  },
  {
    value: "SELECT * FROM shadow WHERE thingId = {thingId}",
    label: "Select Specified Thing",
    autoTrigger: false,
  },
  {
    value: "SELECT * FROM shadow WHERE createdAt > '{time}'",
    label: "By Created Time",
    autoTrigger: false,
  },
  {
    value: "SELECT * FROM shadow WHERE `tags.{tagName}` = '{tagValue}'",
    label: "By Tag Name",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.desired.{propName}` = '{propValue}'",
    label: "By Desired Prop",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.reported.{propName}` = '{propValue}'",
    label: "By Peported Prop",
    autoTrigger: false,
  },
];
