aws iam create-instance-profile \
    --instance-profile-name KarpenterInstanceProfile

aws iam add-role-to-instance-profile \
    --instance-profile-name KarpenterInstanceProfile \
    --role-name KarpenterInstanceNodeRole
