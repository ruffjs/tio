export const suggestions = [
  { value: "SELECT * FROM shadow", autoTrigger: true },
  {
    value:
      "SELECT thingId, connected, `state.reported` as reported, `state.desired` as desired, updatedAt FROM shadow",
    autoTrigger: true,
  },
  {
    value:
      "SELECT thingId, connected, `state.reported`, createdAt as created_time, updatedAt as updated_time, `tags` FROM shadow",
    autoTrigger: true,
  },
  // {
  //   value: "SELECT * FROM shadow WHERE connected = 'true'",
  //   autoTrigger: true,
  // },
  // {
  //   value: "SELECT * FROM shadow WHERE connected = 'false'",
  //   autoTrigger: true,
  // },
  {
    value: "SELECT * FROM shadow WHERE thingId = {thingId}",
    autoTrigger: false,
  },
  {
    value: "SELECT * FROM shadow WHERE `tags.{tagName}` = '{tagValue}'",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.desired.{propName}` = '{propValue}'",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.reported.{propName}` = '{propValue}'",
    autoTrigger: false,
  },
];
