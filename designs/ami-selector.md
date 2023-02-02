# Background

The `AMISelector` field of the [`v1alpha1` `AWSNodeTemplate` resource](/pkg/apis/v1alpha1/awsnodetemplate.go)
is a key-value map with some special key handling (e.g. `aws-ids` is used to pass image IDs).

Most relevant to this design document was the `name` field, which was passed as the `name` filter to
[`DescribeImages` API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html)
call. This field was removed as a special case in #2978, as there is a risk that without filtering
by owner, AMI impersonation could happen with various risks (e.g. security vulnerabilities, malware
such as mining, or cost implications).

While the `Name` tag is still usable, AMI tags are private to an account (so if you want
to use the AWS bottlerocket AMIs, you'd need to tag them yourself in each account that
you use)

# Solutions

1. Restore the previous `name` special case as `aws::name` and add a new `aws::owners`
special case that is passed as the `Owners` argument to `DescribeImages` (this arguments supports both aliases
such as `self` and `amazon` as well as AWS account IDs). Make `self,amazon` the default
for this solution. `aws-ids` also becomes available as `aws::ids` - the `aws::` prefixes
ensure that there won't be clashes with existing AMI tags.

2. Create a v1alpha2 AMISelector with a much more flexible type than go's `map[string]string` and
deprecate v1alpha1. The new AMISelector could have an interface much closer to `DescribeImages`.

An example of this might be:

```
amiSelector:
  owners:
    - self
    - 1234567890
  name: my-ami
  imageids:
    - ami-abcd1234
    - ami-2345cdef
  filters:
    tag:Version: v1.2.3,v1.2.4
    tag:ThisShouldExist:
```

I think there's still some discussion for what this should look like, and how closely it should
map to DescribeImages (here I've taken the approach that the top-level fields all map to an
argument to DescribeImages)

`owners` would default to `self,amazon`

3. Add an `AMIOwners` list field to v1alpha1 (default `["self","amazon"])` and restore `name` as a special case.

# Recommendations

Solution 1 is pretty much already implemented with #3204 - it's very straightforward, but does
create a new special case of `owners` that would conflict with any existing use of `owners` as
an AMI tag.

Solution 3 provides a workaround to the `owners` tag conflict but seems unnecessary and inelegant
just to provide a way to avoid a theoretical conflict.

Solution 2 is the most work, but removes any of the special cases, and provides a much more flexible
approach to AMI filtering in general - by allowing filters other than `tag:`, users will gain
more power.

# Decision from Working Group meeting

Implement Option 1 as a short-term fix to the immediate problem, and implement Option 2 as part
of a bigger API change that addresses similar improvements to security group and subnet selectors.
