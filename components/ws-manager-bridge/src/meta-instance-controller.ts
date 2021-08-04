import { inject, injectable } from "inversify";
import { WorkspaceDB } from "../../gitpod-db/lib/workspace-db";
import { MessageBusIntegration } from "./messagebus-integration";
import { Configuration } from "./config";
import { log } from "../../gitpod-protocol/lib/util/logging";

@injectable()
export class MetaInstanceController {
    @inject(Configuration)
    protected readonly config: Configuration;

    @inject(MessageBusIntegration)
    protected readonly messagebus: MessageBusIntegration;

    @inject(WorkspaceDB)
    protected readonly workspaceDB: WorkspaceDB;

    public async start() {
        const instances = await this.workspaceDB.findRunningInstancesWithWorkspaces(this.config.installation);

        await Promise.all(instances.map(async instance => {
            const creationTime = new Date(instance.latestInstance.creationTime).getTime();
            const preparingKillTime = Date.now() - (this.config.timeouts.preparingPhaseSeconds * 1000);
            const unknownKillTime = Date.now() - (this.config.timeouts.unknownPhaseSeconds * 1000);

            if (creationTime < preparingKillTime || creationTime < unknownKillTime) {
                log.info('Setting workspace instance to stopped', {
                    instanceId: instance.latestInstance.id,
                    creationTime,
                    preparingKillTime,
                    unknownKillTime
                });

                instance.latestInstance.status.phase = 'stopped';

                await this.workspaceDB.storeInstance(instance.latestInstance);
                await this.messagebus.notifyOnInstanceUpdate({}, instance.workspace.ownerId, instance.latestInstance);
            }
        }));
    }
}