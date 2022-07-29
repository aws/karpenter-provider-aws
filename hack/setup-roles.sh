SCRIPT_DIR=website/content/en/preview/getting-started/getting-started-with-eksctl/scripts
TEMP_DIR=$(mktemp -d)

install -m 0777 "${SCRIPT_DIR}/step03-iam-cloud-formation.sh" "$TEMP_DIR/step03.sh"
install -m 0777 "${SCRIPT_DIR}/step04-grant-access.sh" "$TEMP_DIR/step04.sh"
install -m 0777 "$SCRIPT_DIR/step05-controller-iam.sh" "$TEMP_DIR/step05.sh"
install -m 0777 "$SCRIPT_DIR/step06-add-spot-role.sh" "$TEMP_DIR/step06.sh"

"$TEMP_DIR"/step03.sh
"$TEMP_DIR"/step04.sh
"$TEMP_DIR"/step05.sh
"$TEMP_DIR"/step06.sh

rm -r "$TEMP_DIR"